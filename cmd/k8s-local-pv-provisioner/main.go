package main

import "gopkg.in/src-d/go-cli.v0"

var (
	name    = "k8s-local-pv-provisioner"
	version = "undefined"
	build   = "undefined"
)

var app = cli.New(name, version, build, "A service to create local paths for local PVs in Kubernetes")

func main() {
	app.RunMain()
}
