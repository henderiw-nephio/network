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

package endpoints

import (
	"sort"
	"strings"

	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type Endpoints struct {
	*invv1alpha1.EndpointList
}

func GetKeys(s *metav1.LabelSelector) []string {
	keys := []string{}
	for k := range s.MatchLabels {
		keys = append(keys, k)
	}
	for _, req := range s.MatchExpressions {
		if req.Operator == metav1.LabelSelectorOpExists || req.Operator == metav1.LabelSelectorOpIn {
			keys = append(keys, req.Key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (r *Endpoints) iterator() *iterator[invv1alpha1.Endpoint] {
	return &iterator[invv1alpha1.Endpoint]{curIdx: -1, items: r.Items}
}

// GetUniqueKeyValues returns unique values based on the selector keys
// e.g. used to return the unique clusters endpoints if the selector is about clusters
func (r *Endpoints) GetUniqueKeyValues(selector labels.Selector, keys []string) []string {
	values := []string{}
	lvalues := map[string]struct{}{}

	iter := r.iterator()
	for iter.HasNext() {
		if selector.Matches(labels.Set(iter.Value().Labels)) {
			selectorName := getKeyValueName(labels.Set(iter.Value().Labels), keys)
			if _, ok := lvalues[selectorName]; !ok {
				values = append(values, selectorName)
			}
			lvalues[selectorName] = struct{}{}
		}
	}
	return values
}

// getKeyValueName provides a unique name based on all keys (keys are sorted)
func getKeyValueName(labels map[string]string, keys []string) string {
	var sb strings.Builder
	for i, key := range keys {
		if v, ok := labels[key]; ok {
			if i == 0 {
				sb.WriteString(v)
			} else {
				sb.WriteString("-" + v)
			}
		}
	}
	return sb.String()
}

func (r *Endpoints) GetNodes(selector labels.Selector) []string {
	values := []string{}
	lvalues := map[string]struct{}{}

	iter := r.iterator()
	for iter.HasNext() {
		if selector.Matches(labels.Set(iter.Value().Labels)) {
			if _, ok := lvalues[iter.Value().Spec.NodeName]; !ok {
				values = append(values, iter.Value().Spec.NodeName)
			}
			lvalues[iter.Value().Spec.NodeName] = struct{}{}
		}
	}
	return values
}

func (r *Endpoints) GetSelectorEndpoints(s *metav1.LabelSelector) (map[string][]invv1alpha1.Endpoint, error) {
	selector, err := metav1.LabelSelectorAsSelector(s)
	if err != nil {
		return nil, err
	}
	keys := GetKeys(s)
	epPerSelector := map[string][]invv1alpha1.Endpoint{}

	iter := r.iterator()
	for iter.HasNext() {
		if selector.Matches(labels.Set(iter.Value().Labels)) {
			selectorName := getKeyValueName(labels.Set(iter.Value().Labels), keys)
			// initialize struct with the selectorName if it does not exist
			if _, ok := epPerSelector[selectorName]; !ok {
				epPerSelector[selectorName] = []invv1alpha1.Endpoint{}
			}
			epPerSelector[selectorName] = append(epPerSelector[selectorName], iter.Value())
		}
	}
	return epPerSelector, nil
}
