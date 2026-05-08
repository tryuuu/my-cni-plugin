package networkpolicy

import (
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

type EventType string

const (
	EventPodAdded      EventType = "pod-added"
	EventPodUpdated    EventType = "pod-updated"
	EventPodDeleted    EventType = "pod-deleted"
	EventPolicyChanged EventType = "policy-changed"
	EventNSChanged     EventType = "ns-changed"
	EventFullResync    EventType = "full-resync"
)

type Event struct {
	Type      EventType
	PodIPs    []string
	Namespace string
}

type Enqueuer interface {
	Enqueue(event Event)
}

func isPodReady(pod *corev1.Pod) bool {
	return pod.Status.PodIP != ""
}

func enqueue(q Enqueuer, event Event) {
	if q != nil {
		q.Enqueue(event)
	}
}

func SetupWatcher(factory informers.SharedInformerFactory, s Store, q Enqueuer) {
	podInformer := factory.Core().V1().Pods().Informer()
	nsInformer := factory.Core().V1().Namespaces().Informer()
	policyInformer := factory.Networking().V1().NetworkPolicies().Informer()

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			if !isPodReady(pod) {
				return
			}
			s.AddOrUpdatePod(pod)
			fmt.Printf("[pod] added: %s (%s/%s)\n", pod.Status.PodIP, pod.Namespace, pod.Name)
			enqueue(q, Event{Type: EventPodAdded, PodIPs: []string{pod.Status.PodIP}})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)
			if !isPodReady(newPod) {
				s.DeletePod(oldPod)
				return
			}
			s.AddOrUpdatePod(newPod)
			fmt.Printf("[pod] updated: %s (%s/%s)\n", newPod.Status.PodIP, newPod.Namespace, newPod.Name)
			enqueue(q, Event{Type: EventPodUpdated, PodIPs: []string{newPod.Status.PodIP}})
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				pod, ok = tombstone.Obj.(*corev1.Pod)
				if !ok {
					return
				}
			}
			ip := pod.Status.PodIP
			s.DeletePod(pod)
			fmt.Printf("[pod] deleted: %s (%s/%s)\n", ip, pod.Namespace, pod.Name)
			if ip != "" {
				enqueue(q, Event{Type: EventPodDeleted, PodIPs: []string{ip}})
			}
		},
	})

	nsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns := obj.(*corev1.Namespace)
			s.AddOrUpdateNamespace(ns)
			fmt.Printf("[namespace] added: %s\n", ns.Name)
			enqueue(q, Event{Type: EventNSChanged, Namespace: ns.Name})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNS := oldObj.(*corev1.Namespace)
			newNS := newObj.(*corev1.Namespace)
			if reflect.DeepEqual(oldNS.Labels, newNS.Labels) {
				return
			}
			s.AddOrUpdateNamespace(newNS)
			fmt.Printf("[namespace] labels updated: %s\n", newNS.Name)
			enqueue(q, Event{Type: EventNSChanged, Namespace: newNS.Name})
		},
		DeleteFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				ns, ok = tombstone.Obj.(*corev1.Namespace)
				if !ok {
					return
				}
			}
			s.DeleteNamespace(ns.Name)
			fmt.Printf("[namespace] deleted: %s\n", ns.Name)
		},
	})

	policyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			np := obj.(*networkingv1.NetworkPolicy)
			s.AddOrUpdatePolicy(np)
			fmt.Printf("[policy] added: %s/%s\n", np.Namespace, np.Name)
			enqueue(q, Event{Type: EventPolicyChanged, Namespace: np.Namespace})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			np := newObj.(*networkingv1.NetworkPolicy)
			s.AddOrUpdatePolicy(np)
			fmt.Printf("[policy] updated: %s/%s\n", np.Namespace, np.Name)
			enqueue(q, Event{Type: EventPolicyChanged, Namespace: np.Namespace})
		},
		DeleteFunc: func(obj interface{}) {
			np, ok := obj.(*networkingv1.NetworkPolicy)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				np, ok = tombstone.Obj.(*networkingv1.NetworkPolicy)
				if !ok {
					return
				}
			}
			s.DeletePolicy(np.Namespace, np.Name)
			fmt.Printf("[policy] deleted: %s/%s\n", np.Namespace, np.Name)
			enqueue(q, Event{Type: EventPolicyChanged, Namespace: np.Namespace})
		},
	})
}
