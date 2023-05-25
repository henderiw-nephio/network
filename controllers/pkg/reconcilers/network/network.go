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
	"context"
	"fmt"

	infrav1alpha1 "github.com/henderiw-nephio/network/apis/infra/v1alpha1"

	//ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/hash"
	"github.com/srl-labs/ygotsrl/v22"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type network struct {
	*infrav1alpha1.Network
	devices   map[string]*ygotsrl.Device
	resources map[corev1.ObjectReference]client.Object
	eps       *endpoints
	hash      hash.HashTable
}

func (self *network) PopulateBridgeDomains(ctx context.Context) error {
	for _, bd := range self.Spec.BridgeDomains {
		for _, itfce := range bd.Interfaces {
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
			fmt.Println("selector", selector)
			selectedEndpoints, err := self.eps.GetEndpointsPerSelector(selector)
			if err != nil {
				return err
			}
			fmt.Println("selectedEndpoints", selectedEndpoints)
			for selectorName, eps := range selectedEndpoints {
				for _, ep := range eps {
					// selectorName is a global unique identity
					// create a VLANDatabase (selectorName)
					// create a BridgeDomain (bdName + "-" + selectorName + "bd")
					// create BD Index (hash)
					// allocate VLAN ID
					bdName := fmt.Sprintf("%s-bd", bd.Name)
					if itfce.Selector != nil {
						bdName = fmt.Sprintf("%s-%s-bd", bd.Name, selectorName)
					}

					vlanId, err := self.PopulateBridgeDomain(ctx, ep.Spec.NodeName, selectorName, bdName)
					if err != nil {
						return err
					}
					// create interface/subinterface
					// create networkInstance interface
					self.PopulateBridgeInterface(ctx, bdName, vlanId, ep)
				}
			}
		}
	}
	return nil
}

func (self *network) PopulateRoutingTables(ctx context.Context) error {
	for _, rt := range self.Spec.RoutingTables {
		o := ipamv1alpha1.BuildNetworkInstance(metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-rt", rt.Name),
			Namespace:       self.Namespace,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: self.APIVersion, Kind: self.Kind, Name: self.Name, UID: self.UID, Controller: pointer.Bool(true)}},
		}, ipamv1alpha1.NetworkInstanceSpec{
			Prefixes: rt.Prefixes,
		}, ipamv1alpha1.NetworkInstanceStatus{})
		self.resources[corev1.ObjectReference{APIVersion: o.GetResourceVersion(), Kind: o.GetObjectKind().GroupVersionKind().Kind, Name: o.GetName(), Namespace: o.GetNamespace()}] = o

		for _, itfce := range rt.Interfaces {
			if itfce.Kind == infrav1alpha1.InterfaceKindBridgeDomain {
				// create IRB + we need to lookup the selectors in the bridge domain
				for _, bd := range self.Spec.BridgeDomains {
					if itfce.BridgeDomainName != nil && bd.Name == *itfce.BridgeDomainName {
						for _, itfce := range bd.Interfaces {
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
							selectedEndpoints, err := self.eps.GetEndpointsPerSelector(selector)
							if err != nil {
								return err
							}

							for selectorName, eps := range selectedEndpoints {
								for _, ep := range eps {
									bdName := fmt.Sprintf("%s-bd", bd.Name)
									if itfce.Selector != nil {
										bdName = fmt.Sprintf("%s-%s-bd", bd.Name, selectorName)
									}

									self.PopulateIRBInterface(ctx, false, bdName, fmt.Sprintf("%s-rt", rt.Name), ep)
									self.PopulateIRBInterface(ctx, true, bdName, fmt.Sprintf("%s-rt", rt.Name), ep)
								}
							}
						}
					}
				}
				return nil
			}
			// non irb interfaces
			selector := &metav1.LabelSelector{}
			if itfce.Selector != nil {
				selector = itfce.Selector
			} else {
				selector = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						invv1alpha1.NephioInterfaceNameKey: *itfce.InterfaceName,
						invv1alpha1.NephioNodeNameKey:      *itfce.NodeName,
					},
				}
			}
			selectedEndpoints, err := self.eps.GetEndpointsPerSelector(selector)
			if err != nil {
				return err
			}
			for selectorName, eps := range selectedEndpoints {
				for _, ep := range eps {
					// create a VLANDB (selectorName) -> to be done earlier
					// create BD Index (hash)
					// allocate VLAN ID
					rtName := fmt.Sprintf("%s-rt", rt.Name)
					vlanId, err := self.PopulateRoutingInstance(ctx, ep.Spec.NodeName, selectorName, rtName)
					if err != nil {
						return err
					}

					// create interface/subinterface
					// create networkInstance interface
					self.PopulateRoutedInterface(ctx, rtName, vlanId, ep)
				}
			}
		}
	}
	return nil
}
