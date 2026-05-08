package networkpolicy

import (
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
)

type portSpec struct {
	Port     int32
	Protocol Protocol
}

func resolvePorts(ports []networkingv1.NetworkPolicyPort) []portSpec {
	if len(ports) == 0 {
		return []portSpec{{Port: 0, Protocol: ""}}
	}
	result := make([]portSpec, 0, len(ports))
	for _, p := range ports {
		var port int32
		if p.Port != nil {
			port = int32(p.Port.IntValue())
		}
		var proto Protocol
		if p.Protocol != nil {
			proto = Protocol(*p.Protocol)
		}
		result = append(result, portSpec{Port: port, Protocol: proto})
	}
	return result
}

type Protocol string

const (
	ProtocolTCP  Protocol = "TCP"
	ProtocolUDP  Protocol = "UDP"
	ProtocolSCTP Protocol = "SCTP"
)

type Rule struct {
	CIDR     string
	Port     int32
	Protocol Protocol
}

type PolicyRules struct {
	PodIP          string
	IngressRules   []Rule
	EgressRules    []Rule
	IngressDefault bool
	EgressDefault  bool
}

type Evaluator interface {
	Evaluate(podIP string) (*PolicyRules, error)
}

type evaluator struct {
	store Store
}

func NewEvaluator(store Store) Evaluator {
	return &evaluator{store: store}
}

func (e *evaluator) Evaluate(podIP string) (*PolicyRules, error) {
	targetPod, ok := e.store.GetPodMeta(podIP)
	if !ok {
		return nil, fmt.Errorf("pod not found: %s", podIP)
	}

	policies := e.store.ListPoliciesByNamespace(targetPod.Namespace)

	var applicable []networkingv1.NetworkPolicy
	for _, policy := range policies {
		if matchesSelector(targetPod.Labels, &policy.Spec.PodSelector) {
			applicable = append(applicable, policy)
		}
	}

	rules := &PolicyRules{PodIP: podIP}

	for _, policy := range applicable {
		for _, pt := range policy.Spec.PolicyTypes {
			if pt == networkingv1.PolicyTypeIngress {
				rules.IngressDefault = true
			}
			if pt == networkingv1.PolicyTypeEgress {
				rules.EgressDefault = true
			}
		}
	}

	for _, policy := range applicable {
		for _, entry := range policy.Spec.Ingress {
			var ips []string
			if len(entry.From) == 0 {
				ips = []string{"0.0.0.0/0"}
			} else {
				for _, peer := range entry.From {
					ips = append(ips, resolvePeer(peer, targetPod.Namespace, e.store)...)
				}
			}
			for _, ip := range ips {
				for _, ps := range resolvePorts(entry.Ports) {
					rules.IngressRules = append(rules.IngressRules, Rule{CIDR: ip, Port: ps.Port, Protocol: ps.Protocol})
				}
			}
		}

		for _, entry := range policy.Spec.Egress {
			var ips []string
			if len(entry.To) == 0 {
				ips = []string{"0.0.0.0/0"}
			} else {
				for _, peer := range entry.To {
					ips = append(ips, resolvePeer(peer, targetPod.Namespace, e.store)...)
				}
			}
			for _, ip := range ips {
				for _, ps := range resolvePorts(entry.Ports) {
					rules.EgressRules = append(rules.EgressRules, Rule{CIDR: ip, Port: ps.Port, Protocol: ps.Protocol})
				}
			}
		}
	}

	return rules, nil
}
