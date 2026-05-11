package networkpolicy

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

type Reconciler struct {
	store     Store
	evaluator Evaluator
	ipt       IPTablesManager
	queue     workqueue.RateLimitingInterface //nolint:staticcheck
}

func NewReconciler(store Store, evaluator Evaluator, ipt IPTablesManager) *Reconciler {
	return &Reconciler{
		store:     store,
		evaluator: evaluator,
		ipt:       ipt,
		queue:     workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()), //nolint:staticcheck
	}
}

func (r *Reconciler) Enqueue(event Event) {
	r.queue.Add(&event)
}

func (r *Reconciler) Run(stopCh <-chan struct{}) {
	defer r.queue.ShutDown()
	e := Event{Type: EventFullResync}
	r.queue.Add(&e)
	go wait.Until(r.worker, time.Second, stopCh)
	<-stopCh
}

func (r *Reconciler) worker() {
	for r.processNextItem() {
	}
}

func (r *Reconciler) processNextItem() bool {
	item, quit := r.queue.Get()
	if quit {
		return false
	}
	defer r.queue.Done(item)

	event := item.(*Event)
	if err := r.reconcile(*event); err != nil {
		fmt.Printf("[reconciler] error: %v, requeuing\n", err)
		r.queue.AddRateLimited(item)
		return true
	}
	r.queue.Forget(item)
	return true
}

func (r *Reconciler) reconcile(event Event) error {
	if event.Type == EventFullResync {
		return r.fullResync()
	}

	if event.Type == EventPodDeleted {
		for _, ip := range event.PodIPs {
			if err := r.ipt.DeletePod(ip); err != nil {
				return err
			}
		}
		return nil
	}

	podIPs := event.PodIPs
	if event.Type == EventPolicyChanged || event.Type == EventNSChanged {
		pods := r.store.ListPodsByNamespace(event.Namespace)
		podIPs = make([]string, 0, len(pods))
		for _, pod := range pods {
			podIPs = append(podIPs, pod.IP)
		}
	}

	for _, ip := range podIPs {
		rules, err := r.evaluator.Evaluate(ip)
		if err != nil {
			return err
		}
		if err := r.ipt.SyncPod(rules); err != nil {
			return err
		}
		fmt.Printf("[reconciler] synced pod %s\n", ip)
	}
	return nil
}

func (r *Reconciler) fullResync() error {
	allPods := r.store.ListAllPods()
	fmt.Printf("[reconciler] full resync: %d pods\n", len(allPods))
	for _, pod := range allPods {
		rules, err := r.evaluator.Evaluate(pod.IP)
		if err != nil {
			return err
		}
		if err := r.ipt.SyncPod(rules); err != nil {
			return err
		}
		fmt.Printf("[reconciler] synced pod %s\n", pod.IP)
	}
	return nil
}
