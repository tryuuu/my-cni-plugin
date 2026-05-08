package networkpolicy

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type PodMeta struct {
	IP        string
	Namespace string
	Labels    map[string]string
}

type Store interface {
	AddOrUpdatePod(pod *corev1.Pod)
	DeletePod(pod *corev1.Pod)
	GetPodMeta(ip string) (PodMeta, bool)
	ListPodsByNamespace(ns string) []PodMeta
	ListAllPods() []PodMeta

	AddOrUpdateNamespace(ns *corev1.Namespace)
	DeleteNamespace(name string)
	GetNamespaceLabels(name string) map[string]string
	ListNamespaces() []string

	AddOrUpdatePolicy(np *networkingv1.NetworkPolicy)
	DeletePolicy(ns, name string)
	ListPoliciesByNamespace(ns string) []networkingv1.NetworkPolicy
}

type store struct {
	mu       sync.RWMutex
	podByIP  map[string]PodMeta
	nsLabels map[string]map[string]string
	policies map[string]map[string]networkingv1.NetworkPolicy
}

func NewStore() Store {
	return &store{
		podByIP:  make(map[string]PodMeta),
		nsLabels: make(map[string]map[string]string),
		policies: make(map[string]map[string]networkingv1.NetworkPolicy),
	}
}

func (s *store) AddOrUpdatePod(pod *corev1.Pod) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.podByIP[pod.Status.PodIP] = PodMeta{
		IP:        pod.Status.PodIP,
		Namespace: pod.Namespace,
		Labels:    pod.Labels,
	}
}

func (s *store) DeletePod(pod *corev1.Pod) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pod.Status.PodIP == "" {
		return
	}
	delete(s.podByIP, pod.Status.PodIP)
}

func (s *store) GetPodMeta(ip string) (PodMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, ok := s.podByIP[ip]
	return meta, ok
}

func (s *store) ListPodsByNamespace(ns string) []PodMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []PodMeta
	for _, meta := range s.podByIP {
		if meta.Namespace == ns {
			result = append(result, meta)
		}
	}
	return result
}

func (s *store) ListAllPods() []PodMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PodMeta, 0, len(s.podByIP))
	for _, meta := range s.podByIP {
		result = append(result, meta)
	}
	return result
}

func (s *store) AddOrUpdateNamespace(ns *corev1.Namespace) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nsLabels[ns.Name] = ns.Labels
}

func (s *store) DeleteNamespace(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.nsLabels, name)
}

func (s *store) GetNamespaceLabels(name string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.nsLabels[name]
}

func (s *store) ListNamespaces() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0, len(s.nsLabels))
	for ns := range s.nsLabels {
		result = append(result, ns)
	}
	return result
}

func (s *store) AddOrUpdatePolicy(np *networkingv1.NetworkPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.policies[np.Namespace] == nil {
		s.policies[np.Namespace] = make(map[string]networkingv1.NetworkPolicy)
	}
	s.policies[np.Namespace][np.Name] = *np
}

func (s *store) DeletePolicy(ns, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if nsMap, ok := s.policies[ns]; ok {
		delete(nsMap, name)
	}
}

func (s *store) ListPoliciesByNamespace(ns string) []networkingv1.NetworkPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nsMap, ok := s.policies[ns]
	if !ok {
		return nil
	}
	result := make([]networkingv1.NetworkPolicy, 0, len(nsMap))
	for _, np := range nsMap {
		result = append(result, np)
	}
	return result
}
