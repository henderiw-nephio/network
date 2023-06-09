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

package resources

import (
	"context"
	"sync"

	"github.com/nephio-project/nephio/controllers/pkg/resource"
	"github.com/nokia/k8s-ipam/pkg/meta"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Resources interface {
	AddNewResource(o client.Object)
	GetExistingResources(ctx context.Context) error
	APIApply(ctx context.Context) error
	GetNewResources() map[corev1.ObjectReference]client.Object
}

type Config struct {
	CR             client.Object
	MatchingLabels client.MatchingLabels
	Owns           []schema.GroupVersionKind
}

func New(c resource.APIPatchingApplicator, cfg Config) Resources {
	return &resources{
		APIPatchingApplicator: c,
		cfg:                   cfg,
		newResources:          map[corev1.ObjectReference]client.Object{},
		existingResources:     map[corev1.ObjectReference]client.Object{},
	}
}

type resources struct {
	resource.APIPatchingApplicator
	cfg               Config
	m                 sync.RWMutex
	newResources      map[corev1.ObjectReference]client.Object
	existingResources map[corev1.ObjectReference]client.Object
}

func (r *resources) AddNewResource(o client.Object) {
	r.m.Lock()
	defer r.m.Unlock()
	r.newResources[corev1.ObjectReference{
		APIVersion: o.GetResourceVersion(),
		Kind:       o.GetObjectKind().GroupVersionKind().Kind,
		Namespace:  o.GetNamespace(),
		Name:       o.GetName(),
	}] = o
}

func (r *resources) GetExistingResources(ctx context.Context) error {
	r.m.Lock()
	defer r.m.Unlock()

	for _, gvk := range r.cfg.Owns {
		objs := meta.GetUnstructuredListFromGVK(&gvk)

		opts := []client.ListOption{
			r.cfg.MatchingLabels,
		}

		if err := r.List(ctx, objs, opts...); err != nil {
			return err
		}
		for _, o := range objs.Items {
			for _, ref := range o.GetOwnerReferences() {
				if ref.UID == r.cfg.CR.GetUID() {
					r.existingResources[corev1.ObjectReference{APIVersion: o.GetAPIVersion(), Kind: o.GetKind(), Name: o.GetName(), Namespace: o.GetNamespace()}] = &o
				}
			}
		}
	}
	return nil
}

// APIApply
// step 1. remove the exisiting resources that overlap with the new resources
// step 2. delete the exisiting resources that are no longer needed
// step 3. apply the new resources to the api server
func (r *resources) APIApply(ctx context.Context) error {
	r.m.Lock()
	defer r.m.Unlock()

	for ref := range r.newResources {
		delete(r.existingResources, ref)
	}

	// step2. delete the exisiting resource that are no longer needed
	for _, o := range r.existingResources {
		if err := r.Delete(ctx, o); err != nil {
			return err
		}
	}

	// step3. apply the new resources to the api server
	for _, o := range r.newResources {
		if err := r.Apply(ctx, o); err != nil {
			return err
		}
	}
	return nil
}

func (r *resources) GetNewResources() map[corev1.ObjectReference]client.Object {
	r.m.RLock()
	defer r.m.RUnlock()

	return r.newResources
}
