package main

import (
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/tryuuu/my-cni-plugin/pkg/networkpolicy"
)

func main() {
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

	store := networkpolicy.NewStore()
	factory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
	networkpolicy.SetupWatcher(factory, store, nil)

	stopCh := make(chan struct{})
	factory.Start(stopCh)
	cache.WaitForCacheSync(
		stopCh,
		factory.Core().V1().Pods().Informer().HasSynced,
		factory.Core().V1().Namespaces().Informer().HasSynced,
		factory.Networking().V1().NetworkPolicies().Informer().HasSynced,
	)
	fmt.Println("network-policy-controller started")
	<-stopCh
}
