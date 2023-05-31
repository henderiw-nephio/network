/*
Copyright 2023 The Nephio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package network

/*
import (
	"context"

	"github.com/go-logr/logr"
	infrav1alpha1 "github.com/henderiw-nephio/network/apis/infra/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type EnqueueRequestForAllEndpoints struct {
	client client.Client
	l      logr.Logger
	ctx    context.Context
}

// Create enqueues a request for all ip allocation within the ipam
func (e *EnqueueRequestForAllEndpoints) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (e *EnqueueRequestForAllEndpoints) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.ObjectOld, q)
	e.add(evt.ObjectNew, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (e *EnqueueRequestForAllEndpoints) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (e *EnqueueRequestForAllEndpoints) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

func (e *EnqueueRequestForAllEndpoints) add(obj runtime.Object, queue adder) {
	cr, ok := obj.(*invv1alpha1.Endpoint)
	if !ok {
		return
	}
	e.l = log.FromContext(e.ctx)
	e.l.Info("event", "kind", obj.GetObjectKind(), "name", cr.GetName())

	networks := &infrav1alpha1.NetworkList{}
	if err := e.client.List(e.ctx, networks); err != nil {
		return
	}

	for _, network := range networks.Items {
		// only enqueue if the provider and the networktopology match
		if cr.Labels[invv1alpha1.NephioProviderKey] == nokiaSRLProvider &&
			cr.Labels[invv1alpha1.NephioTopologyKey] == network.Spec.Topology {
			e.l.Info("event requeue network", "name", network.GetName())
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: network.GetNamespace(),
				Name:      network.GetName()}})
		}
	}
}
*/