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

import "sync"

type Tracker interface {
	IsDone(nodeName string, selectorName string) bool
}

func NewTracker() Tracker {
	return &tracker{t: map[string]map[string]struct{}{}}
}

type tracker struct {
	m sync.Mutex
	t map[string]map[string]struct{}
}

func (r *tracker) IsDone(nodeName string, selectorName string) bool {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.t[nodeName]; !ok {
		r.t[nodeName] = map[string]struct{}{}
		r.t[nodeName][selectorName] = struct{}{}
		return false
	}
	if _, ok := r.t[nodeName][selectorName]; !ok {
		r.t[nodeName][selectorName] = struct{}{}
		return false
	}
	return true
}
