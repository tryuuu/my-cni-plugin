package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/vishvananda/netlink"
)

type NetConf struct {
	types.NetConf
	BridgeName string `json:"bridgeName"`
	MTU        int    `json:"mtu"`
}

func loadConf(data []byte) (*NetConf, error) {
	conf := &NetConf{}
	if err := json.Unmarshal(data, conf); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if conf.BridgeName == "" {
		conf.BridgeName = "cni0"
	}
	if conf.MTU == 0 {
		conf.MTU = 1500
	}
	return conf, nil
}

func CmdAdd(args *skel.CmdArgs) error {
	conf, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}

	ipamResult, err := invoke.DelegateAdd(context.TODO(), conf.IPAM.Type, args.StdinData, nil)
	if err != nil {
		return fmt.Errorf("IPAM ADD failed: %w", err)
	}
	result, err := types100.NewResultFromResult(ipamResult)
	if err != nil {
		return fmt.Errorf("failed to convert IPAM result: %w", err)
	}

	br, err := ensureBridge(conf.BridgeName, conf.MTU)
	if err != nil {
		return err
	}

	hostVethLink, err := setupVeth(args, result, conf.MTU)
	if err != nil {
		return err
	}

	if err := attachToBridge(hostVethLink, br); err != nil {
		return err
	}

	if err := setBridgeGateway(br, result); err != nil {
		return err
	}

	brLink, _ := netlink.LinkByName(conf.BridgeName)
	result.Interfaces = []*types100.Interface{
		{Name: conf.BridgeName, Mac: brLink.Attrs().HardwareAddr.String()},
		{Name: hostVethLink.Attrs().Name, Mac: hostVethLink.Attrs().HardwareAddr.String()},
		{Name: args.IfName, Sandbox: args.Netns},
	}
	contIfIdx := 2
	for i := range result.IPs {
		result.IPs[i].Interface = &contIfIdx
	}

	return types.PrintResult(result, conf.CNIVersion)
}

func CmdCheck(args *skel.CmdArgs) error {
	if _, err := loadConf(args.StdinData); err != nil {
		return err
	}
	return fmt.Errorf("CHECK not implemented")
}

func CmdDel(args *skel.CmdArgs) error {
	conf, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}

	if err := invoke.DelegateDel(context.TODO(), conf.IPAM.Type, args.StdinData, nil); err != nil {
		return fmt.Errorf("IPAM DEL failed: %w", err)
	}

	return teardownVeth(args)
}
