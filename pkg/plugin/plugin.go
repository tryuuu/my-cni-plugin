package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
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
	return nil
}
