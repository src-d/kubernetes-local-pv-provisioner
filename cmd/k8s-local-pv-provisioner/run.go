package main

import (
	"context"
	"errors"
	"os"
	"path"
	"strings"

	"github.com/apex/log"

	gocli "gopkg.in/src-d/go-cli.v0"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	app.AddCommand(&RunCommand{})
}

type RunCommand struct {
	gocli.PlainCommand `name:"run" short-description:"run a watcher for PVs" long-description:"Run an in-cluster watcher for PVs and create the needed paths if needed"`
	NodeName           string `long:"node-name" required:"true" env:"NODE_NAME" description:"Hostname of the current node the pod runs on"`
	KubernetesContext  string `long:"context" env:"KUBERNETES_CONTEXT" description:"If set the program will load the kubernetes configuration from a kubeconfig file for the given context"`
	RootfsPath         string `long:"rootfs-path" env:"ROOTFS_PATH" default:"/rootfs" description:"Path to the mounted root file system of the node"`
}

func (r *RunCommand) ExecuteContext(ctx context.Context, args []string) error {
	client, err := r.getClientSet()
	if err != nil {
		return err
	}

	watcher, err := client.CoreV1().PersistentVolumes().Watch(meta_v1.ListOptions{})
	if err != nil {
		return err
	}
	watcher.ResultChan()
L:
	for {
		select {
		case <-ctx.Done():
			watcher.Stop()
			break L
		case event := <-watcher.ResultChan():
			if event.Type == watch.Error {
				return errors.New("Got kubernetes watch error")
			}
			if event.Type == watch.Added || event.Type == watch.Modified {
				r.setUpPV(event.Object.(*core_v1.PersistentVolume))
			}
		}
	}

	return nil
}

func (r *RunCommand) getClientSet() (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if r.KubernetesContext != "" {
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{
				CurrentContext: r.KubernetesContext,
			},
		).ClientConfig()
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func (r *RunCommand) setUpPV(pv *core_v1.PersistentVolume) {
	if pv.Spec.Local == nil {
		log.Infof("%s is not a local volume, skipping", pv.ObjectMeta.Name)
		return
	}

	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil || pv.Spec.NodeAffinity.Required.NodeSelectorTerms == nil {
		log.Infof("%s does not have correct NodeAffinity, skipping", pv.ObjectMeta.Name)
		return
	}

	matches := false
	for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, matchExpression := range term.MatchExpressions {
			if matchExpression.Key != "kubernetes.io/hostname" { // we only have hostname support
				continue
			}
			if matchExpression.Operator == core_v1.NodeSelectorOperator("In") {
				for _, value := range matchExpression.Values {
					if value == r.NodeName {
						matches = true
					}
				}
			} else if matchExpression.Operator == core_v1.NodeSelectorOperator("NotIn") {
				matches = true
				for _, value := range matchExpression.Values {
					if value == r.NodeName {
						matches = false
					}
				}
			}

		}
	}

	if matches {
		pvPath := path.Join(r.RootfsPath, pv.Spec.Local.Path)
		_, err := os.Stat(pvPath)
		if err != nil && strings.Contains(err.Error(), "no such file or directory") {
			err = os.MkdirAll(pvPath, 0755)
			if err != nil {
				log.Errorf("Failed to create directory \"%s\": %s", pvPath, err.Error())
				return
			}

			log.Infof("Successfully created directory \"%s\" for %s", pvPath, pv.ObjectMeta.Name)
		} else if err != nil {
			log.Errorf("Failed stat directory \"%s\": %s", pvPath, err.Error())
		}
	}
}
