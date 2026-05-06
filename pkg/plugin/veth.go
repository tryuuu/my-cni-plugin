package plugin

import (
	"crypto/sha1"
	"fmt"

	"github.com/containernetworking/cni/pkg/skel"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func hostVethName(containerID, ifName string) string {
	h := sha1.Sum([]byte(containerID + ifName))
	return fmt.Sprintf("veth%x", h[:4])
}

func setupVeth(args *skel.CmdArgs, result *types100.Result, mtu int) (netlink.Link, error) {
	hostVeth := hostVethName(args.ContainerID, args.IfName)
	contTmp := hostVeth + "p"

	if err := netlink.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostVeth, MTU: mtu, TxQLen: -1},
		PeerName:  contTmp,
	}); err != nil {
		return nil, fmt.Errorf("failed to add veth pair: %w", err)
	}

	hostVethLink, err := netlink.LinkByName(hostVeth)
	if err != nil {
		return nil, fmt.Errorf("failed to find host veth: %w", err)
	}
	contVethLink, err := netlink.LinkByName(contTmp)
	if err != nil {
		return nil, fmt.Errorf("failed to find container veth: %w", err)
	}

	containerNS, err := ns.GetNS(args.Netns)
	if err != nil {
		return nil, fmt.Errorf("failed to open netns %q: %w", args.Netns, err)
	}
	defer containerNS.Close()

	if err := netlink.LinkSetNsFd(contVethLink, int(containerNS.Fd())); err != nil {
		return nil, fmt.Errorf("failed to move veth into container ns: %w", err)
	}

	if err := containerNS.Do(func(_ ns.NetNS) error {
		iface, err := netlink.LinkByName(contTmp)
		if err != nil {
			return fmt.Errorf("failed to find %q in container ns: %w", contTmp, err)
		}
		if err := netlink.LinkSetName(iface, args.IfName); err != nil {
			return fmt.Errorf("failed to rename veth to %q: %w", args.IfName, err)
		}
		iface, err = netlink.LinkByName(args.IfName)
		if err != nil {
			return fmt.Errorf("failed to find %q after rename: %w", args.IfName, err)
		}
		for _, ip := range result.IPs {
			if err := netlink.AddrAdd(iface, &netlink.Addr{IPNet: &ip.Address}); err != nil {
				return fmt.Errorf("failed to add IP %s: %w", ip.Address, err)
			}
		}
		if err := netlink.LinkSetUp(iface); err != nil {
			return fmt.Errorf("failed to set %q up: %w", args.IfName, err)
		}
		for _, ip := range result.IPs {
			if ip.Gateway == nil {
				continue
			}
			if err := netlink.RouteAdd(&netlink.Route{
				LinkIndex: iface.Attrs().Index,
				Scope:     netlink.Scope(0),
				Gw:        ip.Gateway,
			}); err != nil {
				return fmt.Errorf("failed to add default route via %s: %w", ip.Gateway, err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return hostVethLink, nil
}

func teardownVeth(args *skel.CmdArgs) error {
	if args.Netns == "" {
		return nil
	}
	containerNS, err := ns.GetNS(args.Netns)
	if err != nil {
		return nil
	}
	defer containerNS.Close()

	return containerNS.Do(func(_ ns.NetNS) error {
		iface, err := netlink.LinkByName(args.IfName)
		if err != nil {
			return nil
		}
		return netlink.LinkDel(iface)
	})
}
