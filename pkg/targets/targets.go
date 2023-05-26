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

package targets

import (
	"sync"

	"github.com/openconfig/gnmic/target"
	"k8s.io/apimachinery/pkg/types"
)

type Target interface {
	Set(nsn types.NamespacedName, t *target.Target)
	Get(nsn types.NamespacedName) *target.Target
}

func New() Target {
	return &t{
		targets: map[types.NamespacedName]*target.Target{},
	}
}

type t struct {
	m       sync.RWMutex
	targets map[types.NamespacedName]*target.Target
}

func (r *t) Get(nsn types.NamespacedName) *target.Target {
	r.m.RLock()
	defer r.m.RUnlock()

	t, ok := r.targets[nsn]
	if !ok {
		return nil
	}
	return t
}

func (r *t) Set(nsn types.NamespacedName, t *target.Target) {
	r.m.Lock()
	defer r.m.Unlock()

	r.targets[nsn] = t
}
