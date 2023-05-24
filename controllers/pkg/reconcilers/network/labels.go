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

import (
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
)

func getClusterLabels(labels map[string]string, clusterName string) map[string]string {
	l := map[string]string{}
	for k, v := range labels {
		l[k] = v
	}
	l[invv1alpha1.NephioClusterNameKey] = clusterName
	// update labels - defaulting to prefixkind = network and gateway true
	l[allocv1alpha1.NephioGatewayKey] = "true"
	return l
}
