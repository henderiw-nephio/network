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
	"github.com/henderiw-nephio/network/pkg/endpoints"
	"github.com/henderiw-nephio/network/pkg/ipam"
	"github.com/henderiw-nephio/network/pkg/nodes"
	"github.com/henderiw-nephio/network/pkg/resources"
	"github.com/henderiw-nephio/network/pkg/vlan"
	reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/hash"
	"github.com/pkg/errors"

	//"github.com/nokia/k8s-ipam/pkg/resource"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	"github.com/srl-labs/ygotsrl/v22"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultNetworkName = "default"
)

type network struct {
	resource.APIPatchingApplicator
	apply     bool
	devices   map[string]*ygotsrl.Device
	resources resources.Resources
	eps       *endpoints.Endpoints
	nodes     *nodes.Nodes
	hash      hash.HashTable
	ipam      ipam.IPAM
	vlan      vlan.VLAN
}

func (r *network) populateIPAMNetworkInstance(cr *infrav1alpha1.Network, rt infrav1alpha1.RoutingTable) client.Object {
	// create IPAM NetworkInstance
	o := r.ipam.ClaimIPAMDB(cr, rt.Name, rt.Prefixes)
	r.resources.AddNewResource(o)
	return o
}

func (r *network) populateVlanDatabase(cr *infrav1alpha1.Network, dbIndexName string) client.Object {
	// create VLAN DataBase
	o := r.vlan.ClaimVLANDB(cr, dbIndexName)
	r.resources.AddNewResource(o)
	return o
}

func (r *network) PopulateBridgeDomains(ctx context.Context, cr *infrav1alpha1.Network) error {
	for _, bd := range cr.Spec.BridgeDomains {
		// tracker tracks if we already initialized the context
		// it ensure we dont duplicate allocations, etc etc
		tr := NewTracker()
		for _, itfce := range bd.Interfaces {
			// GetSelectorEndpoints returns a map with key = selectorName and value list of endpoints
			// associated to that selector
			// A selectorName can be a grouping like a cluster or a nodeName/InterfaceName
			selectedEndpoints, err := r.eps.GetSelectorEndpoints(endpoints.GetSelector(itfce))
			if err != nil {
				msg := fmt.Sprintf("cannot get endpoints from selector: %v", endpoints.GetSelector(itfce))
				return errors.Wrap(err, msg)
			}
			for selectorName, eps := range selectedEndpoints {
				for _, ep := range eps {
					if !tr.IsAlreadyDone(ep.Spec.NodeName, selectorName) {
						// selectorName is a global unique identity (interface/node or a grouping like clusters)
						bdName := itfce.GetBridgeDomainName(bd.Name, selectorName)

						// create a VLANDatabase (based on selectorName)
						if itfce.AttachmentType == reqv1alpha1.AttachmentTypeVLAN {
							r.populateVlanDatabase(cr, selectorName)
						}
						// we dont proceed if we need to apply the databases first, since they are used
						// as an index to allocate resources from
						if r.apply {
							continue
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

func (r *network) PopulateRoutingTables(ctx context.Context, cr *infrav1alpha1.Network) error {
	for _, rt := range cr.Spec.RoutingTables {
		r.populateIPAMNetworkInstance(cr, rt)

		// we need to identify which nodes and interfaces belong to this routing table
		for _, itfce := range rt.Interfaces {
			if itfce.Kind == infrav1alpha1.InterfaceKindBridgeDomain {
				// create IRB + we lookup the interfaces/selectors in the bridge domain
				for _, bd := range cr.Spec.BridgeDomains {
					if itfce.BridgeDomainName != nil && bd.Name == *itfce.BridgeDomainName {
						// we dont proceed if we need to apply the databases first, since they are used
						// as an index to allocate resources from
						if r.apply {
							continue
						}
						// tracker tracks if we already initialized the context
						// it ensure we dont duplicate allocations, etc etc
						tr := NewTracker()
						for _, itfce := range bd.Interfaces {
							selectedEndpoints, err := r.eps.GetSelectorEndpoints(endpoints.GetSelector(itfce))
							if err != nil {
								msg := fmt.Sprintf("cannot get endpoints from selector: %v", endpoints.GetSelector(itfce))
								return errors.Wrap(err, msg)
							}
							for selectorName, eps := range selectedEndpoints {
								for _, ep := range eps {
									if !tr.IsAlreadyDone(ep.Spec.NodeName, selectorName) {
										rtName := rt.Name
										// selectorName is a global unique identity (interface/node or a grouping like clusters)
										bdName := itfce.GetBridgeDomainName(bd.Name, selectorName)
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

			selectedEndpoints, err := r.eps.GetSelectorEndpoints(endpoints.GetSelector(itfce))
			if err != nil {
				return err
			}
			for selectorName, eps := range selectedEndpoints {
				for _, ep := range eps {
					rtName := rt.Name
					if !tr.IsAlreadyDone(ep.Spec.NodeName, ep.Spec.InterfaceName) {

						if itfce.AttachmentType == reqv1alpha1.AttachmentTypeVLAN {
							r.populateVlanDatabase(cr, selectorName)

						}
						if r.apply {
							// we can return here since we do another stage
							continue
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
		if rt.Name == defaultNetworkName {
			for _, node := range r.nodes.GetNodes() {
				if err := r.PopulateNode(ctx, cr, node.Name, rt.Name, rt.Prefixes); err != nil {
					return errors.Wrap(err, "cannot populate node")
				}
			}
		}
	}
	return nil
}
