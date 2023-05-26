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
	reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	"github.com/nephio-project/nephio/controllers/pkg/resource"

	//ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/hash"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/srl-labs/ygotsrl/v22"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type network struct {
	*infrav1alpha1.Network
	resource.APIPatchingApplicator
	apply           bool
	devices         map[string]*ygotsrl.Device
	resources       map[corev1.ObjectReference]client.Object
	eps             *endpoints
	hash            hash.HashTable
	IpamClientProxy clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation]
	VlanClientProxy clientproxy.Proxy[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation]
}

func (r *network) populateIPAMNetworkInstance(rt infrav1alpha1.RoutingTable) client.Object {
	// create VLAN DataBase
	o := ipamv1alpha1.BuildNetworkInstance(
		metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-rt", rt.Name),
			Namespace:       r.Namespace,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: r.APIVersion, Kind: r.Kind, Name: r.Name, UID: r.UID, Controller: pointer.Bool(true)}},
		}, ipamv1alpha1.NetworkInstanceSpec{
			Prefixes: rt.Prefixes,
		}, ipamv1alpha1.NetworkInstanceStatus{})
	r.resources[corev1.ObjectReference{APIVersion: o.GetResourceVersion(), Kind: o.GetObjectKind().GroupVersionKind().Kind, Name: o.GetName(), Namespace: o.GetNamespace()}] = o
	return o
}

func (r *network) populateVlanDatabase(selectorName string) client.Object {
	// create VLAN DataBase
	o := vlanv1alpha1.BuildVLANDatabase(
		metav1.ObjectMeta{
			Name:            selectorName, // the vlan db is always the selectorName since the bd is physical and not virtual
			Namespace:       r.Namespace,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: r.APIVersion, Kind: r.Kind, Name: r.Name, UID: r.UID, Controller: pointer.Bool(true)}},
		},
		vlanv1alpha1.VLANDatabaseSpec{
			Kind: vlanv1alpha1.VLANDBKindESG,
		},
		vlanv1alpha1.VLANDatabaseStatus{},
	)
	r.resources[corev1.ObjectReference{APIVersion: o.APIVersion, Kind: o.Kind, Name: o.Name, Namespace: o.Namespace}] = o
	return o
}

func (r *network) PopulateBridgeDomains(ctx context.Context) error {
	for _, bd := range r.Spec.BridgeDomains {
		for _, itfce := range bd.Interfaces {
			selectedEndpoints, err := r.eps.GetEndpointsPerSelector(getSelector(itfce))
			if err != nil {
				return err
			}
			for selectorName, eps := range selectedEndpoints {
				for _, ep := range eps {
					// selectorName is a global unique identity (interface/node or a grouping like clusters)
					bdName := fmt.Sprintf("%s-bd", bd.Name)
					if itfce.Selector != nil {
						bdName = fmt.Sprintf("%s-%s-bd", bd.Name, selectorName)
					}

					// create a VLANDatabase (based on selectorName)
					if itfce.AttachmentType == reqv1alpha1.AttachmentTypeVLAN {
						o := r.populateVlanDatabase(selectorName)
						if r.apply {
							if err := r.Apply(ctx, o); err != nil {
								return err
							}
							// we can continue here since we do another stage
							continue
						}
					} else {
						if r.apply {
							continue
						}
					}

					// create bridgedomain (bdname) + create a bd index
					r.PopulateBridgeDomain(ctx, ep.Spec.NodeName, selectorName, bdName)
					// create interface/subinterface + networkInstance interface
					if err := r.PopulateBridgeInterface(ctx, selectorName, bdName, ep, itfce.AttachmentType); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func getSelector(itfce infrav1alpha1.Interface) *metav1.LabelSelector {
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

func (r *network) PopulateRoutingTables(ctx context.Context) error {
	for _, rt := range r.Spec.RoutingTables {
		o := r.populateIPAMNetworkInstance(rt)
		if r.apply {
			if err := r.Apply(ctx, o); err != nil {
				return err
			}
		}

		for _, itfce := range rt.Interfaces {
			if itfce.Kind == infrav1alpha1.InterfaceKindBridgeDomain {
				// create IRB + we lookup the interfaces/selectors in the bridge domain
				for _, bd := range r.Spec.BridgeDomains {
					if itfce.BridgeDomainName != nil && bd.Name == *itfce.BridgeDomainName {
						if r.apply {
							continue
						}
						// tracker tracks if we already initialized the context
						// it ensure we dont duplicate allocations, etc etc
						tr := NewTracker()
						for _, itfce := range bd.Interfaces {
							selectedEndpoints, err := r.eps.GetEndpointsPerSelector(getSelector(itfce))
							if err != nil {
								return err
							}
							for selectorName, eps := range selectedEndpoints {
								for _, ep := range eps {
									if !tr.IsDone(ep.Spec.NodeName, "dummmy") {
										rtName := fmt.Sprintf("%s-rt", rt.Name)
										bdName := fmt.Sprintf("%s-bd", bd.Name)
										if itfce.Selector != nil {
											bdName = fmt.Sprintf("%s-%s-bd", bd.Name, selectorName)
										}
										r.PopulateIRBInterface(ctx, false, bdName, rtName, ep, rt.Prefixes, getSelectorLabels(ep.Labels, getKeys(getSelector(itfce))))
										r.PopulateIRBInterface(ctx, true, bdName, rtName, ep, rt.Prefixes, getSelectorLabels(ep.Labels, getKeys(getSelector(itfce))))
									}
								}
							}
						}
					}
				}
				return nil
			}
			// non irb interfaces
			selectedEndpoints, err := r.eps.GetEndpointsPerSelector(getSelector(itfce))
			if err != nil {
				return err
			}
			for selectorName, eps := range selectedEndpoints {
				for _, ep := range eps {
					// create a VLANDB (selectorName) -> to be done earlier
					// create BD Index (hash)
					// allocate VLAN ID
					rtName := fmt.Sprintf("%s-rt", rt.Name)

					if itfce.AttachmentType == reqv1alpha1.AttachmentTypeVLAN {
						o := r.populateVlanDatabase(selectorName)
						if r.apply {
							if err := r.Apply(ctx, o); err != nil {
								return err
							}
							// we can return here since we do another stage
							continue
						}
					} else {
						if r.apply {
							// we can return here since we do another stage
							continue
						}
					}

					r.PopulateRoutingInstance(ctx, ep.Spec.NodeName, selectorName, rtName)
					// create interface/subinterface +  networkInstance interface
					if err := r.PopulateRoutedInterface(ctx, selectorName, rtName, ep, itfce.AttachmentType); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
