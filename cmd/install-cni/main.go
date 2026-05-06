package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type cniConfig struct {
	CNIVersion string     `json:"cniVersion"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	BridgeName string     `json:"bridgeName"`
	MTU        int        `json:"mtu"`
	IPAM       ipamConfig `json:"ipam"`
}

type ipamConfig struct {
	Type    string `json:"type"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

func main() {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		fmt.Fprintln(os.Stderr, "NODE_NAME not set")
		os.Exit(1)
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get in-cluster config: %v\n", err)
		os.Exit(1)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create clientset: %v\n", err)
		os.Exit(1)
	}

	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get node %s: %v\n", nodeName, err)
		os.Exit(1)
	}
	if node.Spec.PodCIDR == "" {
		fmt.Fprintln(os.Stderr, "spec.podCIDR is empty")
		os.Exit(1)
	}

	gw, err := gatewayFromCIDR(node.Spec.PodCIDR)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to derive gateway: %v\n", err)
		os.Exit(1)
	}

	conf := cniConfig{
		CNIVersion: "1.0.0",
		Name:       "my-cni",
		Type:       "my-cni",
		BridgeName: "cni0",
		MTU:        1500,
		IPAM: ipamConfig{
			Type:    "host-local",
			Subnet:  node.Spec.PodCIDR,
			Gateway: gw,
		},
	}

	data, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal config: %v\n", err)
		os.Exit(1)
	}

	if err := copyBinary("/my-cni", "/opt/cni/bin/my-cni"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to install binary: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("installed /opt/cni/bin/my-cni")

	const confPath = "/etc/cni/net.d/05-my-cni.conf"
	if err := os.WriteFile(confPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", confPath, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (subnet: %s, gateway: %s)\n", confPath, node.Spec.PodCIDR, gw)
}

func copyBinary(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = out.ReadFrom(in)
	return err
}

func gatewayFromCIDR(cidr string) (string, error) {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return "", fmt.Errorf("IPv6 not supported")
	}
	ip4[3]++
	return ip4.String(), nil
}
