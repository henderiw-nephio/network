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
	"net/netip"
	"strconv"
	"strings"

	reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/nokia/k8s-ipam/pkg/utils/util"
	"github.com/openconfig/ygot/ygot"
	"github.com/srl-labs/ygotsrl/v22"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func (r *network) PopulateBridgeInterface(ctx context.Context, selectorName, bdName string, ep invv1alpha1.Endpoint, attachmentType reqv1alpha1.AttachmentType) error {
	nodeName := ep.Spec.NodeName
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = new(ygotsrl.Device)
	}

	vlanId := uint16(0)
	if attachmentType == reqv1alpha1.AttachmentTypeVLAN {
		vlanAlloc, err := r.VlanClientProxy.Allocate(ctx, vlanv1alpha1.BuildVLANAllocation(
			metav1.ObjectMeta{
				Name:      bdName,
				Namespace: r.Namespace,
			},
			vlanv1alpha1.VLANAllocationSpec{
				VLANDatabase: corev1.ObjectReference{Name: selectorName, Namespace: r.Namespace},
			},
			vlanv1alpha1.VLANAllocationStatus{},
		), nil)
		if err != nil {
			return err
		}
		vlanId = *vlanAlloc.Status.VLANID
	}

	niItfceSubItfceName := strings.Join([]string{ep.Spec.InterfaceName, strconv.Itoa(int(vlanId))}, ".")

	i := r.devices[nodeName].GetOrCreateInterface(ep.Spec.InterfaceName)
	si := i.GetOrCreateSubinterface(uint32(vlanId))
	si.Type = ygotsrl.SrlNokiaInterfaces_SiType_bridged
	si.Vlan = &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan{
		Encap: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap{
			SingleTagged: &ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Vlan_Encap_SingleTagged{
				VlanId: ygotsrl.UnionUint16(vlanId),
			},
		},
	}
	ni := r.devices[nodeName].GetOrCreateNetworkInstance(bdName)
	ni.GetOrCreateInterface(niItfceSubItfceName)
	return nil
}

func (r *network) PopulateRoutedInterface(ctx context.Context, selectorName, rtName string, ep invv1alpha1.Endpoint, attachmentType reqv1alpha1.AttachmentType) error {
	nodeName := ep.Spec.NodeName
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = new(ygotsrl.Device)
	}
	vlanId := uint16(0)
	if attachmentType == reqv1alpha1.AttachmentTypeVLAN {
		// allocate vlanID -> vlanDatabase = selectorName, allocName = bdName or rtName
		vlanAlloc, err := r.VlanClientProxy.Allocate(ctx, vlanv1alpha1.BuildVLANAllocation(
			metav1.ObjectMeta{
				Name:      rtName,
				Namespace: r.Namespace,
			},
			vlanv1alpha1.VLANAllocationSpec{
				VLANDatabase: corev1.ObjectReference{Name: selectorName, Namespace: r.Namespace},
			},
			vlanv1alpha1.VLANAllocationStatus{},
		), nil)
		if err != nil {
			return err
		}
		vlanId = *vlanAlloc.Status.VLANID
	}
	// allocate IP = per link (label in ep)
	// allocate Address based on the link
	// how to know it is ipv4 or ipv6

	niItfceSubItfceName := strings.Join([]string{ep.Spec.InterfaceName, strconv.Itoa(int(vlanId))}, ".")
	i := r.devices[nodeName].GetOrCreateInterface(ep.Spec.InterfaceName)
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
	ni := r.devices[nodeName].GetOrCreateNetworkInstance(rtName)
	ni.GetOrCreateInterface(niItfceSubItfceName)
	return nil
}

const irbInterfaceName = "irb0"

func (r *network) PopulateIRBInterface(ctx context.Context, routed bool, bdName, rtName string, ep invv1alpha1.Endpoint, prefixes []ipamv1alpha1.Prefix, labels map[string]string) error {
	nodeName := ep.Spec.NodeName
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = new(ygotsrl.Device)
	}
	// allocate IP = per bdName (label in ep)
	// allocate Address based on the bdName
	// how to know it is ipv4 or ipv6

	niIndex := r.hash.Insert(bdName, "dummy", map[string]string{})
	niItfceSubItfceName := strings.Join([]string{irbInterfaceName, strconv.Itoa(int(niIndex))}, ".")
	i := r.devices[nodeName].GetOrCreateInterface(irbInterfaceName)
	si := i.GetOrCreateSubinterface(niIndex)

	if routed {
		for _, prefix := range prefixes {
			pi := iputil.NewPrefixInfo(netip.MustParsePrefix(prefix.Prefix))

			prefixKind := ipamv1alpha1.PrefixKindNetwork
			if k, ok := prefix.Labels[allocv1alpha1.NephioPrefixKindKey]; ok {
				prefixKind = ipamv1alpha1.GetPrefixKindFromString(k)
			}
			prefixLength := 24
			if pi.IsIpv6() {
				prefixLength = 64
			}
			if prefixKind == ipamv1alpha1.PrefixKindPool {
				prefixLength = 16
				if pi.IsIpv6() {
					prefixLength = 48
				}
			}
			af := pi.GetAddressFamily()
			_, err := r.IpamClientProxy.Allocate(ctx, ipamv1alpha1.BuildIPAllocation(
				metav1.ObjectMeta{
					Name:      bdName,
					Namespace: r.Namespace,
				},
				ipamv1alpha1.IPAllocationSpec{
					Kind:            prefixKind,
					NetworkInstance: corev1.ObjectReference{Name: rtName, Namespace: r.Namespace},
					AddressFamily:   &af,
					PrefixLength:    util.PointerUint8(prefixLength),
					CreatePrefix:    pointer.Bool(true),
					AllocationLabels: allocv1alpha1.AllocationLabels{
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: labels,
						},
					},
				},
				ipamv1alpha1.IPAllocationStatus{},
			), nil)
			if err != nil {
				return err
			}
			// for network based prefixes i will allocate a gateway IP
			if prefixKind == ipamv1alpha1.PrefixKindNetwork {
				prefixAlloc, err := r.IpamClientProxy.Allocate(ctx, ipamv1alpha1.BuildIPAllocation(
					metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-gateway", bdName),
						Namespace: r.Namespace,
					},
					ipamv1alpha1.IPAllocationSpec{
						Kind:            prefixKind,
						NetworkInstance: corev1.ObjectReference{Name: rtName, Namespace: r.Namespace},
						AddressFamily:   &af,
						AllocationLabels: allocv1alpha1.AllocationLabels{
							Selector: &metav1.LabelSelector{
								MatchLabels: labels,
							},
						},
					},
					ipamv1alpha1.IPAllocationStatus{},
				), nil)
				if err != nil {
					return err
				}
				if pi.IsIpv6() {
					ipv6 := si.GetOrCreateIpv6()
					ipv6.AppendAddress(&ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_Address{
						AnycastGw: ygot.Bool(true),
						IpPrefix:  prefixAlloc.Status.Prefix,
					})
					ipv6.GetOrCreateNeighborDiscovery().LearnUnsolicited = ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv6_NeighborDiscovery_LearnUnsolicited_global
					ipv6.NeighborDiscovery.GetOrCreateHostRoute().GetOrCreatePopulate(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_HostRoute_Populate_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
					ipv6.NeighborDiscovery.GetOrCreateEvpn().GetOrCreateAdvertise(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
				} else {
					ipv4 := si.GetOrCreateIpv4()
					ipv4.AppendAddress(&ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Address{
						AnycastGw: ygot.Bool(true),
						IpPrefix:  prefixAlloc.Status.Prefix,
					})
					ipv4.GetOrCreateArp().LearnUnsolicited = ygot.Bool(true)
					ipv4.Arp.GetOrCreateHostRoute().GetOrCreatePopulate(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_HostRoute_Populate_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
					ipv4.Arp.GetOrCreateEvpn().GetOrCreateAdvertise(ygotsrl.E_SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType(ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Arp_Evpn_Advertise_RouteType_dynamic))
				}
				si.GetOrCreateAnycastGw().VirtualRouterId = ygot.Uint8(1)
			}

		}

	}
	ni := r.devices[nodeName].GetOrCreateNetworkInstance(bdName)
	if routed {
		ni = r.devices[nodeName].GetOrCreateNetworkInstance(rtName)
	}
	ni.GetOrCreateInterface(niItfceSubItfceName)
	return nil
}
