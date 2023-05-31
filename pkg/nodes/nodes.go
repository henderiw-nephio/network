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

package nodes

import (
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
)

type Nodes struct {
	*invv1alpha1.NodeList
}

func (r *Nodes) iterator() *iterator[invv1alpha1.Node] {
	return &iterator[invv1alpha1.Node]{curIdx: -1, items: r.Items}
}

func (r *Nodes) GetNodes() []invv1alpha1.Node {
	nodes := []invv1alpha1.Node{}

	iter := r.iterator()
	for iter.HasNext() {
		nodes = append(nodes, iter.Value())
	}
	return nodes
}
