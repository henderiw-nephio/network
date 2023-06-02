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

package device

import (
	"strconv"
	"strings"

	
	reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/openconfig/ygot/ygot"
	"github.com/srl-labs/ygotsrl/v22"
)

const (

	IRBInterfaceName    = "irb0"
	SystemInterfaceName = "system0"
)

func getInterfaceName(ifName string) string {
	ifName = strings.ReplaceAll(ifName, "-", "/")
	if ifName[0] == 'e' {
		ifName = strings.ReplaceAll(ifName, "e", "ethernet-")
	}
	return ifName
}

func getNiInterfaceName(ifName string, index int) string {
	return strings.Join([]string{ifName, strconv.Itoa(int(index))}, ".")
}

// AddBridgedInterface adds a bridge interface to the device config
// index is the vlan ID or 0 for untagged bridge interfaces
func (d Device) AddBridgedInterface(niName, ifName string, index int, attachmentType reqv1alpha1.AttachmentType) {
	ifName = getInterfaceName(ifName)
	i := d.GetOrCreateInterface(ifName)
	si := i.GetOrCreateSubinterface(uint32(index))
	si.Type = ygotsrl.SrlNokiaInterfaces_SiType_bridged
	if attachmentType == reqv1alpha1.AttachmentTypeVLAN {
		i.VlanTagging = ygot.Bool(true)
		si.Vlan = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan{
			Encap: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap{
				SingleTagged: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap_SingleTagged{
					VlanId: ygotsrl.UnionUint16(index),
				},
			},
		}
	}
	ni := d.GetOrCreateNetworkInstance(niName)
	ni.Type = ygotsrl.SrlNokiaNetworkInstance_NiType_mac_vrf
	ni.GetOrCreateInterface(getNiInterfaceName(ifName, index))
}

// AddRouteInterface adds a bridge interface to the device config
// index is the vlan ID or 0 for untagged bridge interfaces
func (d *Device) AddRoutedInterface(niName, ifName string, index int, attachmentType reqv1alpha1.AttachmentType, prefixes iputil.PrefixClaims) {
	ifName = getInterfaceName(ifName)
	i := d.GetOrCreateInterface(ifName)
	si := i.GetOrCreateSubinterface(uint32(index))
	si.Type = ygotsrl.SrlNokiaInterfaces_SiType_routed
	if attachmentType == reqv1alpha1.AttachmentTypeVLAN {
		i.VlanTagging = ygot.Bool(true)
		si.Vlan = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan{
			Encap: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap{
				SingleTagged: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap_SingleTagged{
					VlanId: ygotsrl.UnionUint16(index),
				},
			},
		}
	}
	for isIpv6, addresses := range prefixes {
		if isIpv6 {
			ipv6 := si.GetOrCreateIpv6()
			for _, address := range addresses {
				if ifName == IRBInterfaceName {
					ipv6.AppendAddress(&ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_Address{
						AnycastGw: ygot.Bool(true),
						IpPrefix:  address,
					})
					ipv6.GetOrCreateNeighborDiscovery().LearnUnsolicited = ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_NeighborDiscovery_LearnUnsolicited_global
					ipv6.NeighborDiscovery.GetOrCreateHostRoute().GetOrCreatePopulate(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_HostRoute_Populate_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
					ipv6.NeighborDiscovery.GetOrCreateEvpn().GetOrCreateAdvertise(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
				} else {
					ipv6.AppendAddress(&ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_Address{
						IpPrefix: address,
					})
				}
			}
		} else {
			ipv4 := si.GetOrCreateIpv4()
			for _, address := range addresses {
				if ifName == IRBInterfaceName {
					ipv4.AppendAddress(&ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Address{
						AnycastGw: ygot.Bool(true),
						IpPrefix:  address,
					})
					ipv4.GetOrCreateArp().LearnUnsolicited = ygot.Bool(true)
					ipv4.Arp.GetOrCreateHostRoute().GetOrCreatePopulate(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_HostRoute_Populate_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
					ipv4.Arp.GetOrCreateEvpn().GetOrCreateAdvertise(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
				} else {
					ipv4.AppendAddress(&ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Address{
						IpPrefix: address,
					})
				}
			}
		}
	}
	ni := d.GetOrCreateNetworkInstance(niName)
	ni.Type = ygotsrl.SrlNokiaNetworkInstance_NiType_ip_vrf
	if niName == "default" {
		ni.Type = ygotsrl.SrlNokiaNetworkInstance_NiType_default	
	}
	ni.GetOrCreateInterface(getNiInterfaceName(ifName, index))
}
