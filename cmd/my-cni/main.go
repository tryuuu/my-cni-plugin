package main

import (
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/tryuuu/my-cni-plugin/pkg/plugin"
)

func main() {
	skel.PluginMain(plugin.CmdAdd, plugin.CmdCheck, plugin.CmdDel, version.All, "my-cni v0.1")
}
