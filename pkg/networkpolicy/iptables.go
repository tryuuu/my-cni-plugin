package networkpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/coreos/go-iptables/iptables"
)

type IPTablesManager interface {
	EnsureBaseChains() error
	SyncPod(rules *PolicyRules) error
	DeletePod(podIP string) error
}

type iptablesManager struct {
	ipt *iptables.IPTables
}

func NewIPTablesManager() (IPTablesManager, error) {
	ipt, err := iptables.New()
	if err != nil {
		return nil, fmt.Errorf("iptables init: %w", err)
	}
	return &iptablesManager{ipt: ipt}, nil
}

func chainName(prefix, podIP string) string {
	h := sha256.Sum256([]byte(podIP))
	return prefix + hex.EncodeToString(h[:])[:13]
}

func ingressChain(podIP string) string {
	return chainName("KUBE-NWPLCY-IN-", podIP)
}

func egressChain(podIP string) string {
	return chainName("KUBE-NWPLCY-EG-", podIP)
}

const forwardChain = "KUBE-NWPLCY-FORWARD"

func (m *iptablesManager) EnsureBaseChains() error {
	if err := m.ipt.NewChain("filter", forwardChain); err != nil {
		if !isChainExistsError(err) {
			return fmt.Errorf("create chain %s: %w", forwardChain, err)
		}
	}

	exists, err := m.ipt.Exists("filter", "FORWARD", "-j", forwardChain)
	if err != nil {
		return fmt.Errorf("check FORWARD rule: %w", err)
	}
	if !exists {
		if err := m.ipt.Insert("filter", "FORWARD", 1, "-j", forwardChain); err != nil {
			return fmt.Errorf("insert FORWARD rule: %w", err)
		}
	}
	return nil
}

func isChainExistsError(err error) bool {
	e, ok := err.(*iptables.Error)
	return ok && e.ExitStatus() == 1
}

func (m *iptablesManager) SyncPod(rules *PolicyRules) error {
	podIP := rules.PodIP
	ic := ingressChain(podIP)
	ec := egressChain(podIP)

	for _, chain := range []string{ic, ec} {
		if err := m.ipt.NewChain("filter", chain); err != nil {
			if !isChainExistsError(err) {
				return fmt.Errorf("create chain %s: %w", chain, err)
			}
		}
	}

	if err := m.ipt.AppendUnique("filter", forwardChain, "-d", podIP, "-j", ic); err != nil {
		return fmt.Errorf("add ingress jump: %w", err)
	}
	if err := m.ipt.AppendUnique("filter", forwardChain, "-s", podIP, "-j", ec); err != nil {
		return fmt.Errorf("add egress jump: %w", err)
	}

	for _, chain := range []string{ic, ec} {
		if err := m.ipt.ClearChain("filter", chain); err != nil {
			return fmt.Errorf("flush chain %s: %w", chain, err)
		}
	}

	for _, rule := range rules.IngressRules {
		if err := m.ipt.Append("filter", ic, buildRuleArgs("-s", rule)...); err != nil {
			return fmt.Errorf("append ingress rule: %w", err)
		}
	}
	for _, rule := range rules.EgressRules {
		if err := m.ipt.Append("filter", ec, buildRuleArgs("-d", rule)...); err != nil {
			return fmt.Errorf("append egress rule: %w", err)
		}
	}

	if rules.IngressDefault {
		if err := m.ipt.Append("filter", ic, "-j", "DROP"); err != nil {
			return fmt.Errorf("append ingress drop: %w", err)
		}
	}
	if rules.EgressDefault {
		if err := m.ipt.Append("filter", ec, "-j", "DROP"); err != nil {
			return fmt.Errorf("append egress drop: %w", err)
		}
	}

	return nil
}

func buildRuleArgs(direction string, rule Rule) []string {
	args := []string{direction, rule.CIDR}
	if rule.Protocol != "" {
		args = append(args, "-p", strings.ToLower(string(rule.Protocol)))
	}
	if rule.Port > 0 {
		args = append(args, "--dport", fmt.Sprintf("%d", rule.Port))
	}
	args = append(args, "-j", "ACCEPT")
	return args
}

func (m *iptablesManager) DeletePod(podIP string) error {
	ic := ingressChain(podIP)
	ec := egressChain(podIP)

	if err := m.ipt.Delete("filter", forwardChain, "-d", podIP, "-j", ic); err != nil {
		return fmt.Errorf("delete ingress jump: %w", err)
	}
	if err := m.ipt.Delete("filter", forwardChain, "-s", podIP, "-j", ec); err != nil {
		return fmt.Errorf("delete egress jump: %w", err)
	}

	for _, chain := range []string{ic, ec} {
		if err := m.ipt.ClearChain("filter", chain); err != nil {
			return fmt.Errorf("flush chain %s: %w", chain, err)
		}
		if err := m.ipt.DeleteChain("filter", chain); err != nil {
			return fmt.Errorf("delete chain %s: %w", chain, err)
		}
	}

	return nil
}
