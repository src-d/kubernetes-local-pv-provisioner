# Kubernetes Local PV Provisioner

Kubernetes Local PV Provisioner helps you setting up (local persistent volumes)[https://kubernetes.io/docs/concepts/storage/volumes/#local] by looking at the state and creating an empty directory on the filesystem of the correct Kubernetes server if needed.

# Installation

This tool is made to run in cluster as a DaemonSet with the root file system mounted. For testing purposes it can also run locally with a connection to a Kubernetes cluster, however this is of no use since it can not create the directory on the server.

## Kubernetes manifests
This repository provides example manifests file you can use to deploy this. These contain a service account and RBAC configuration for the tool to be able to read the Persistent Volumes. As well as a DaemonSet to deploy this on all servers in a cluster.
```bash
~ $ cd manifests
~ $ kubectl apply -f rbac.yaml
~ $ kubectl apply -f daemonset.yaml
```

## Helm
We also provide a Helm chart in our [Charts repository](https://github.com/src-d/charts). 
```bash
~ $ helm repo add srcd https://src-d.github.io/charts/
~ $ helm install srcd/kubernetes-local-pv-provisioner --set image.tag=v1.0.0
```

# Configuration

* envvar: `NODE_NAME` flag: `--node-name` This is the server's hostname it will look for in the PV's NodeSelector
* envvar: `ROOTFS_PATH` flag: `--rootfs-path` This is the prefix used in the path to locate where the root filesystem is mounted. Default: `/rootfs`
* envvar: `KUBERNETES_CONTEXT` flag: `--context` If this is set it will not attempt to load the in-cluster service account but loads the context value out of `$HOME/.kube/config`

# Contribute

[Contributions](https://github.com/src-d/kubernetes-local-pv-provisioner/issues) are more than welcome, if you are interested please take a look to
our [Contributing Guidelines](CONTRIBUTING.md).

# Code of Conduct

All activities under source{d} projects are governed by the [source{d} code of conduct](.github/CODE_OF_CONDUCT.md).

# License
Apache License Version 2.0, see [LICENSE](LICENSE).
