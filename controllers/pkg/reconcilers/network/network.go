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
	"github.com/henderiw-nephio/network/pkg/resources"
	reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/hash"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/pkg/errors"

	//"github.com/nokia/k8s-ipam/pkg/resource"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	"github.com/srl-labs/ygotsrl/v22"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type network struct {
	resource.APIPatchingApplicator
	apply           bool
	devices         map[string]*ygotsrl.Device
	resources       resources.Resources
	eps             *endpoints
	nodes           *nodes
	hash            hash.HashTable
	IpamClientProxy clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation]
	VlanClientProxy clientproxy.Proxy[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation]
}

func (r *network) populateIPAMNetworkInstance(rt infrav1alpha1.RoutingTable, cr *infrav1alpha1.Network) client.Object {
	// create IPAM NetworkInstance
	o := ipamv1alpha1.BuildNetworkInstance(
		metav1.ObjectMeta{
			Name:            rt.Name,
			Namespace:       cr.Namespace,
			Labels:          getMatchingLabels(cr),
			OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
		}, ipamv1alpha1.NetworkInstanceSpec{
			Prefixes: rt.Prefixes,
		}, ipamv1alpha1.NetworkInstanceStatus{})
	r.resources.AddNewResource(
		corev1.ObjectReference{APIVersion: o.APIVersion, Kind: o.Kind, Name: o.Name, Namespace: o.Namespace},
		o,
	)
	return o
}

func (r *network) populateVlanDatabase(selectorName string, cr *infrav1alpha1.Network) client.Object {
	// create VLAN DataBase
	o := vlanv1alpha1.BuildVLANDatabase(
		metav1.ObjectMeta{
			Name:            selectorName, // the vlan db is always the selectorName since the bd is physical and not virtual
			Namespace:       cr.Namespace,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
		},
		vlanv1alpha1.VLANDatabaseSpec{
			Kind: vlanv1alpha1.VLANDBKindESG,
		},
		vlanv1alpha1.VLANDatabaseStatus{},
	)
	r.resources.AddNewResource(
		corev1.ObjectReference{APIVersion: o.GetResourceVersion(), Kind: o.GetObjectKind().GroupVersionKind().Kind, Name: o.GetName(), Namespace: o.GetNamespace()},
		o,
	)
	return o
}

func (r *network) PopulateBridgeDomains(ctx context.Context, cr *infrav1alpha1.Network) error {
	for _, bd := range cr.Spec.BridgeDomains {
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
					if !tr.IsAlreadyDone(ep.Spec.NodeName, selectorName) {
						// selectorName is a global unique identity (interface/node or a grouping like clusters)
						bdName := fmt.Sprintf("%s-bd", bd.Name)
						if itfce.Selector != nil {
							bdName = fmt.Sprintf("%s-%s-bd", bd.Name, selectorName)
						}

						// create a VLANDatabase (based on selectorName)
						if itfce.AttachmentType == reqv1alpha1.AttachmentTypeVLAN {
							o := r.populateVlanDatabase(selectorName, cr)
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
						if err := r.PopulateBridgeInterface(ctx, cr, selectorName, bdName, ep, itfce.AttachmentType); err != nil {
							return err
						}
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

func (r *network) PopulateRoutingTables(ctx context.Context, cr *infrav1alpha1.Network) error {
	for _, rt := range cr.Spec.RoutingTables {
		o := r.populateIPAMNetworkInstance(rt, cr)
		if r.apply {
			if err := r.Apply(ctx, o); err != nil {
				return err
			}
		}

		for _, itfce := range rt.Interfaces {
			if itfce.Kind == infrav1alpha1.InterfaceKindBridgeDomain {
				// create IRB + we lookup the interfaces/selectors in the bridge domain
				for _, bd := range cr.Spec.BridgeDomains {
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
									if !tr.IsAlreadyDone(ep.Spec.NodeName, selectorName) {
										rtName := rt.Name
										bdName := fmt.Sprintf("%s-bd", bd.Name)
										if itfce.Selector != nil {
											bdName = fmt.Sprintf("%s-%s-bd", bd.Name, selectorName)
										}
										// populate the bridge part
										if err := r.PopulateIRBInterface(ctx, cr, false, bdName, rtName, ep, rt.Prefixes, getSelectorLabels(ep.Labels, getKeys(getSelector(itfce)))); err != nil {
											return err
										}
										// populate the routed part
										if err := r.PopulateIRBInterface(ctx, cr, true, bdName, rtName, ep, rt.Prefixes, getSelectorLabels(ep.Labels, getKeys(getSelector(itfce)))); err != nil {
											return err
										}
									}
								}
							}
						}
					}
				}
				return nil
			}
			// non IRB interfaces

			// tracker tracks if we already initialized the context
			// it ensure we dont duplicate allocations, etc etc
			tr := NewTracker()

			selectedEndpoints, err := r.eps.GetEndpointsPerSelector(getSelector(itfce))
			if err != nil {
				return err
			}
			for selectorName, eps := range selectedEndpoints {
				for _, ep := range eps {
					rtName := rt.Name
					if !tr.IsAlreadyDone(ep.Spec.NodeName, ep.Spec.InterfaceName) {

						if itfce.AttachmentType == reqv1alpha1.AttachmentTypeVLAN {
							o := r.populateVlanDatabase(selectorName, cr)
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
						// create interface/subinterface + networkInstance interface +
						if err := r.PopulateRoutedInterface(ctx, cr, selectorName, rtName, ep, itfce.AttachmentType, rt.Prefixes, getSelectorLabels(ep.Labels, getKeys(getSelector(itfce)))); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func (r *network) PopulateDefault(ctx context.Context, cr *infrav1alpha1.Network) error {
	for _, rt := range cr.Spec.RoutingTables {
		if rt.Name == "default" {
			for _, node := range r.nodes.GetNodes() {
				if err := r.PopulateNode(ctx, cr, node.Name, rt.Name, rt.Prefixes); err != nil {
					return errors.Wrap(err, "cannot populate node")
				}
			}
		}
	}
	return nil
}
