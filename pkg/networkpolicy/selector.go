package networkpolicy

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
)

func matchesSelector(labels map[string]string, sel *metav1.LabelSelector) bool {
	if sel == nil {
		return true
	}
	selector, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return false
	}
	return selector.Matches(k8slabels.Set(labels))
}

func matchNamespaces(sel *metav1.LabelSelector, store Store) []string {
	var matched []string
	for _, ns := range store.ListNamespaces() {
		if matchesSelector(store.GetNamespaceLabels(ns), sel) {
			matched = append(matched, ns)
		}
	}
	return matched
}

func resolvePeer(peer networkingv1.NetworkPolicyPeer, selfNS string, store Store) []string {
	if peer.IPBlock != nil {
		return []string{peer.IPBlock.CIDR}
	}

	// AND 条件: namespaceSelector にマッチする namespace の中から podSelector にも一致する Pod IP のみ返す
	if peer.NamespaceSelector != nil && peer.PodSelector != nil {
		var ips []string
		for _, ns := range matchNamespaces(peer.NamespaceSelector, store) {
			for _, pod := range store.ListPodsByNamespace(ns) {
				if matchesSelector(pod.Labels, peer.PodSelector) {
					ips = append(ips, pod.IP)
				}
			}
		}
		return ips
	}

	if peer.NamespaceSelector != nil {
		var ips []string
		for _, ns := range matchNamespaces(peer.NamespaceSelector, store) {
			for _, pod := range store.ListPodsByNamespace(ns) {
				ips = append(ips, pod.IP)
			}
		}
		return ips
	}

	if peer.PodSelector != nil {
		var ips []string
		for _, pod := range store.ListPodsByNamespace(selfNS) {
			if matchesSelector(pod.Labels, peer.PodSelector) {
				ips = append(ips, pod.IP)
			}
		}
		return ips
	}

	return []string{"0.0.0.0/0"}
}
