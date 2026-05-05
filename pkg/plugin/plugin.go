package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
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
	if _, err := loadConf(args.StdinData); err != nil {
		return err
	}
	return fmt.Errorf("ADD not implemented")
}

func CmdCheck(args *skel.CmdArgs) error {
	if _, err := loadConf(args.StdinData); err != nil {
		return err
	}
	return fmt.Errorf("CHECK not implemented")
}

func CmdDel(args *skel.CmdArgs) error {
	if _, err := loadConf(args.StdinData); err != nil {
		return err
	}
	return fmt.Errorf("DEL not implemented")
}
