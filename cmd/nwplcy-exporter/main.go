package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	dropPackets = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nwplcy_drop_packets_total",
			Help: "Total number of packets dropped by NetworkPolicy per pod",
		},
		[]string{"dst_pod", "dst_namespace", "direction", "node"},
	)

	forwardInRe = regexp.MustCompile(`-d (\S+) -j (KUBE-NWPLCY-IN-\S+)`)
	forwardEgRe = regexp.MustCompile(`-s (\S+) -j (KUBE-NWPLCY-EG-\S+)`)
)

type prevCounters struct {
	mu   sync.Mutex
	vals map[string]int64
}

func (p *prevCounters) delta(key string, current int64) int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	prev, ok := p.vals[key]
	p.vals[key] = current
	if !ok {
		return current
	}
	if current >= prev {
		return current - prev
	}
	return current
}

var prev = &prevCounters{vals: make(map[string]int64)}

func main() {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		fmt.Fprintln(os.Stderr, "NODE_NAME not set")
		os.Exit(1)
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "k8s config: %v\n", err)
		os.Exit(1)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "k8s client: %v\n", err)
		os.Exit(1)
	}

	ipt, err := iptables.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "iptables init: %v\n", err)
		os.Exit(1)
	}

	prometheus.MustRegister(dropPackets)

	go func() {
		for {
			if err := collect(ipt, client, nodeName); err != nil {
				fmt.Fprintf(os.Stderr, "collect error: %v\n", err)
			}
			time.Sleep(30 * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	fmt.Println("nwplcy-exporter listening on :9101")
	if err := http.ListenAndServe(":9101", nil); err != nil {
		fmt.Fprintf(os.Stderr, "http: %v\n", err)
		os.Exit(1)
	}
}

func collect(ipt *iptables.IPTables, client kubernetes.Interface, nodeName string) error {
	pods, err := client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	podByIP := make(map[string][2]string)
	for _, pod := range pods.Items {
		if pod.Status.PodIP != "" {
			podByIP[pod.Status.PodIP] = [2]string{pod.Name, pod.Namespace}
		}
	}

	type chainInfo struct {
		podIP     string
		direction string
	}
	chainMap := make(map[string]chainInfo)

	fwdRules, err := ipt.List("filter", "KUBE-NWPLCY-FORWARD")
	if err != nil {
		return fmt.Errorf("list FORWARD: %w", err)
	}
	for _, rule := range fwdRules {
		if m := forwardInRe.FindStringSubmatch(rule); m != nil {
			chainMap[m[2]] = chainInfo{podIP: strings.Split(m[1], "/")[0], direction: "ingress"}
		}
		if m := forwardEgRe.FindStringSubmatch(rule); m != nil {
			chainMap[m[2]] = chainInfo{podIP: strings.Split(m[1], "/")[0], direction: "egress"}
		}
	}

	for chain, info := range chainMap {
		rows, err := ipt.Stats("filter", chain)
		if err != nil {
			continue
		}
		for _, row := range rows {
			if len(row) < 3 || row[2] != "DROP" {
				continue
			}
			pkts, err := strconv.ParseInt(row[0], 10, 64)
			if err != nil {
				continue
			}
			pod, ok := podByIP[info.podIP]
			if !ok {
				continue
			}
			key := chain + "|" + info.direction
			delta := prev.delta(key, pkts)
			dropPackets.WithLabelValues(pod[0], pod[1], info.direction, nodeName).Add(float64(delta))
		}
	}
	return nil
}
