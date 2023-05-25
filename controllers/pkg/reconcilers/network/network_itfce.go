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
	"strconv"
	"strings"

	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/srl-labs/ygotsrl/v22"
)

func (self *network) PopulateBridgeInterface(ctx context.Context, bdName string, vlanId uint16, ep invv1alpha1.Endpoint) error {
	niItfceSubItfceName := strings.Join([]string{ep.Spec.InterfaceName, strconv.Itoa(int(vlanId))}, ".")

	i := self.devices[ep.Spec.NodeName].GetOrCreateInterface(ep.Spec.InterfaceName)
	si := i.GetOrCreateSubinterface(uint32(vlanId))
	si.Type = ygotsrl.SrlNokiaInterfaces_SiType_bridged
	si.Vlan = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan{
		Encap: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap{
			SingleTagged: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap_SingleTagged{
				VlanId: ygotsrl.UnionUint16(vlanId),
			},
		},
	}
	ni := self.devices[ep.Spec.NodeName].GetOrCreateNetworkInstance(bdName)
	ni.GetOrCreateInterface(niItfceSubItfceName)
	return nil
}

func (self *network) PopulateRoutedInterface(ctx context.Context, rtName string, vlanId uint16, ep invv1alpha1.Endpoint) error {
	// allocate IP = per link (label in ep)
	// allocate Address based on the link
	// how to know it is ipv4 or ipv6

	niItfceSubItfceName := strings.Join([]string{ep.Spec.InterfaceName, strconv.Itoa(int(vlanId))}, ".")
	i := self.devices[ep.Spec.NodeName].GetOrCreateInterface(ep.Spec.InterfaceName)
	si := i.GetOrCreateSubinterface(uint32(vlanId))
	si.Type = ygotsrl.SrlNokiaInterfaces_SiType_routed
	si.Vlan = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan{
		Encap: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap{
			SingleTagged: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap_SingleTagged{
				VlanId: ygotsrl.UnionUint16(vlanId),
			},
		},
	}
	//si.Ipv4
	//si.IPv6
	ni := self.devices[ep.Spec.NodeName].GetOrCreateNetworkInstance(rtName)
	ni.GetOrCreateInterface(niItfceSubItfceName)
	return nil
}

const irbInterfaceName = "irb0"

func (self *network) PopulateIRBInterface(ctx context.Context, routed bool, bdName, rtName string, ep invv1alpha1.Endpoint) error {
	// allocate IP = per bdName (label in ep)
	// allocate Address based on the bdName
	// how to know it is ipv4 or ipv6

	niIndex := self.hash.Insert(bdName, "dummy", map[string]string{})
	niItfceSubItfceName := strings.Join([]string{irbInterfaceName, strconv.Itoa(int(niIndex))}, ".")
	i := self.devices[ep.Spec.NodeName].GetOrCreateInterface(irbInterfaceName)
	_ = i.GetOrCreateSubinterface(niIndex)

	if routed {
		/*
			ipv4.GetOrCreateArp().LearnUnsolicited = ygot.Bool(true)
			ipv4.Arp.GetOrCreateHostRoute().GetOrCreatePopulate(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_HostRoute_Populate_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
			ipv4.Arp.GetOrCreateEvpn().GetOrCreateAdvertise(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))

			ipv6.GetOrCreateNeighborDiscovery().LearnUnsolicited = ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_NeighborDiscovery_LearnUnsolicited_global
			ipv6.NeighborDiscovery.GetOrCreateHostRoute().GetOrCreatePopulate(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_HostRoute_Populate_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
			ipv6.NeighborDiscovery.GetOrCreateEvpn().GetOrCreateAdvertise(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
		*/
	}
	ni := self.devices[ep.Spec.NodeName].GetOrCreateNetworkInstance(bdName)
	if routed {
		ni = self.devices[ep.Spec.NodeName].GetOrCreateNetworkInstance(rtName)
	}
	ni.GetOrCreateInterface(niItfceSubItfceName)
	return nil
}
