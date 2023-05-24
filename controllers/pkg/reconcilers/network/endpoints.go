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
	"sort"

	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
)

type endpoints struct {
	*invv1alpha1.EndpointList
}

func (self *endpoints) GetClusters() []string {
	clusterNames := []string{}
	lclusterNames := map[string]struct{}{}
	for _, e := range self.Items {
		clusterName, ok := e.GetLabels()[invv1alpha1.NephioClusterNameKey]
		if ok {
			if _, ok := lclusterNames[clusterName]; !ok {
				clusterNames = append(clusterNames, clusterName)
			}
			lclusterNames[clusterName] = struct{}{}
		}
	}
	sort.Strings(clusterNames)
	return clusterNames
}

func (self *endpoints) GetClustersPerNode() map[string][]string {
	clusterNamesPerNode := map[string][]string{}
	lclusterNames := map[string]map[string]struct{}{}
	for _, e := range self.Items {
		clusterName, ok := e.GetLabels()[invv1alpha1.NephioClusterNameKey]
		if ok {
			// initialize lclusterNames struct with the nodeName if it does not exist
			if _, ok := lclusterNames[e.Spec.NodeName]; !ok {
				lclusterNames[e.Spec.NodeName] = map[string]struct{}{}
			}
			// initialize clusterNamesPerNode struct with the nodeName if it does not exist
			if _, ok := clusterNamesPerNode[e.Spec.NodeName]; !ok {
				clusterNamesPerNode[e.Spec.NodeName] = []string{}
			}

			if _, ok := lclusterNames[e.Spec.NodeName][clusterName]; !ok {
				clusterNamesPerNode[e.Spec.NodeName] = append(clusterNamesPerNode[e.Spec.NodeName], clusterName)
				sort.Strings(clusterNamesPerNode[e.Spec.NodeName])
			}
			lclusterNames[e.Spec.NodeName][clusterName] = struct{}{}
		}
	}

	return clusterNamesPerNode
}

func (self *endpoints) GetEndpointsPerClusterPerNode() map[string]map[string][]invv1alpha1.Endpoint {
	epPerClusterPerNode := map[string]map[string][]invv1alpha1.Endpoint{}
	for _, e := range self.Items {
		clusterName, ok := e.GetLabels()[invv1alpha1.NephioClusterNameKey]
		if ok {
			// initialize epPerClusterPerNode struct with the nodeName if it does not exist
			if _, ok := epPerClusterPerNode[e.Spec.NodeName]; !ok {
				epPerClusterPerNode[e.Spec.NodeName] = map[string][]invv1alpha1.Endpoint{}
			}
			if _, ok := epPerClusterPerNode[e.Spec.NodeName][clusterName]; !ok {
				epPerClusterPerNode[e.Spec.NodeName][clusterName] = []invv1alpha1.Endpoint{}
			}
			epPerClusterPerNode[e.Spec.NodeName][clusterName] = append(epPerClusterPerNode[e.Spec.NodeName][clusterName], e)
		}
	}
	return epPerClusterPerNode
}
