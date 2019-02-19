package main

import (
	"context"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/apex/log"

	gocli "gopkg.in/src-d/go-cli.v0"
	core_v1 "k8s.io/api/core/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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

	pvInformer := coreinformers.NewPersistentVolumeInformer(client, time.Minute, cache.Indexers{})

	pvInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r.setUpPV(obj.(*core_v1.PersistentVolume))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			r.setUpPV(newObj.(*core_v1.PersistentVolume))
		},
	})

	stop := make(chan struct{})
	defer close(stop)
	go pvInformer.Run(stop)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	stop <- struct{}{}

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
