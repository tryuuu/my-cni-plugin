package main

import (
	"fmt"
	"net"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/vishvananda/netlink"
)

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

	factory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
	nodeInformer := factory.Core().V1().Nodes().Informer()

	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node := obj.(*corev1.Node)
			if node.Name == nodeName || node.Spec.PodCIDR == "" {
				return
			}
			if err := syncRoute(node, true); err != nil {
				fmt.Fprintf(os.Stderr, "add route: %v\n", err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNode := oldObj.(*corev1.Node)
			newNode := newObj.(*corev1.Node)
			if newNode.Name == nodeName {
				return
			}
			oldIP := nodeInternalIP(oldNode)
			newIP := nodeInternalIP(newNode)
			if oldNode.Spec.PodCIDR == newNode.Spec.PodCIDR && oldIP == newIP {
				return
			}
			if oldNode.Spec.PodCIDR != "" && oldIP != "" {
				if err := syncRoute(oldNode, false); err != nil {
					fmt.Fprintf(os.Stderr, "update route (del old): %v\n", err)
				}
			}
			if newNode.Spec.PodCIDR != "" {
				if err := syncRoute(newNode, true); err != nil {
					fmt.Fprintf(os.Stderr, "update route (add new): %v\n", err)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			node := obj.(*corev1.Node)
			if node.Name == nodeName || node.Spec.PodCIDR == "" {
				return
			}
			if err := syncRoute(node, false); err != nil {
				fmt.Fprintf(os.Stderr, "del route: %v\n", err)
			}
		},
	})

	stopCh := make(chan struct{})
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)
	fmt.Println("route-controller started")
	<-stopCh
}

func nodeInternalIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}

func syncRoute(node *corev1.Node, add bool) error {
	nodeIP := nodeInternalIP(node)
	if nodeIP == "" {
		return fmt.Errorf("no internal IP for node %s", node.Name)
	}
	_, dst, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		return fmt.Errorf("invalid podCIDR %q: %w", node.Spec.PodCIDR, err)
	}
	route := &netlink.Route{
		Dst: dst,
		Gw:  net.ParseIP(nodeIP),
	}
	if add {
		if err := netlink.RouteAdd(route); err != nil {
			if os.IsExist(err) {
				return nil
			}
			return fmt.Errorf("RouteAdd %s via %s: %w", node.Spec.PodCIDR, nodeIP, err)
		}
		fmt.Printf("added route %s via %s (%s)\n", node.Spec.PodCIDR, nodeIP, node.Name)
	} else {
		if err := netlink.RouteDel(route); err != nil {
			return fmt.Errorf("RouteDel %s via %s: %w", node.Spec.PodCIDR, nodeIP, err)
		}
		fmt.Printf("deleted route %s via %s (%s)\n", node.Spec.PodCIDR, nodeIP, node.Name)
	}
	return nil
}
