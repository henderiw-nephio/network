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
	infrav1alpha1 "github.com/nephio-project/api/infra/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetSelector(itfce infrav1alpha1.Interface) *metav1.LabelSelector {
	selector := &metav1.LabelSelector{}
	if itfce.Selector != nil {
		selector = itfce.Selector
	} else {
		// we assume the validation happend here that interfaceName and NodeName are not nil
		selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				invv1alpha1.NephioInterfaceNameKey: *itfce.InterfaceName,
				invv1alpha1.NephioNodeNameKey:      *itfce.NodeName,
			},
		}
	}
	return selector
}
