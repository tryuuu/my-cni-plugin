package plugin

import (
	"fmt"
	"net"

	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/vishvananda/netlink"
)

func ensureBridge(name string, mtu int) (*netlink.Bridge, error) {
	l, err := netlink.LinkByName(name)
	if err == nil {
		br, ok := l.(*netlink.Bridge)
		if !ok {
			return nil, fmt.Errorf("%q already exists but is not a bridge", name)
		}
		return br, nil
	}
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:   name,
			MTU:    mtu,
			TxQLen: -1,
		},
	}
	if err := netlink.LinkAdd(br); err != nil {
		return nil, fmt.Errorf("failed to add bridge %q: %w", name, err)
	}
	if err := netlink.LinkSetUp(br); err != nil {
		return nil, fmt.Errorf("failed to set bridge %q up: %w", name, err)
	}
	return br, nil
}

func attachToBridge(link netlink.Link, br *netlink.Bridge) error {
	if err := netlink.LinkSetMaster(link, br); err != nil {
		return fmt.Errorf("failed to attach %q to bridge: %w", link.Attrs().Name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to set %q up: %w", link.Attrs().Name, err)
	}
	return nil
}

func setBridgeGateway(br *netlink.Bridge, result *types100.Result) error {
	for _, ip := range result.IPs {
		if ip.Gateway == nil {
			continue
		}
		addrs, _ := netlink.AddrList(br, 0)
		for _, a := range addrs {
			if a.IP.Equal(ip.Gateway) {
				return nil
			}
		}
		gwNet := &net.IPNet{IP: ip.Gateway, Mask: ip.Address.Mask}
		if err := netlink.AddrAdd(br, &netlink.Addr{IPNet: gwNet}); err != nil {
			return fmt.Errorf("failed to set bridge gateway %s: %w", ip.Gateway, err)
		}
	}
	return nil
}
