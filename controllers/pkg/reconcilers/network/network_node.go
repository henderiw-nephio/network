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

	infrav1alpha1 "github.com/henderiw-nephio/network/apis/infra/v1alpha1"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/openconfig/ygot/ygot"
	"github.com/srl-labs/ygotsrl/v22"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *network) PopulateNode(ctx context.Context, cr *infrav1alpha1.Network, nodeName, rtName string, prefixes []ipamv1alpha1.Prefix) error {
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = new(ygotsrl.Device)
	}
	d := r.devices[nodeName]
	i := d.GetOrCreateInterface("system0")
	//i.AdminState = ygotsrl.SrlNokiaCommon_AdminState_enable
	i.Description = ygot.String("system interface")

	siIndex := 0
	niItfceSubItfceName := strings.Join([]string{irbInterfaceName, strconv.Itoa(int(siIndex))}, ".")
	si := i.GetOrCreateSubinterface(uint32(siIndex))
	//si.AdminState = ygotsrl.SrlNokiaCommon_AdminState_enable
	si.Description = ygot.String(strings.Join([]string{"system0", "infra"}, "-"))

	// create a routing policy
	rp := d.GetOrCreateRoutingPolicy().GetOrCreatePolicy("export-local")
	rp.Name = ygot.String("export-local")

	for _, prefix := range prefixes {
		pi := iputil.NewPrefixInfo(netip.MustParsePrefix(prefix.Prefix))

		prefixKind := ipamv1alpha1.PrefixKindNetwork
		if value, ok := prefix.Labels[allocv1alpha1.NephioPrefixKindKey]; ok {
			prefixKind = ipamv1alpha1.GetPrefixKindFromString(value)
		}

		if prefixKind == ipamv1alpha1.PrefixKindLoopback {
			// add the prefix labels to the prefix selector labels
			prefixSelectorLabels := map[string]string{
				allocv1alpha1.NephioNsnNameKey:      ipamv1alpha1.GetNameFromNetworkInstancePrefix(rtName, pi.String()),
				allocv1alpha1.NephioNsnNamespaceKey: cr.Namespace,
			}

			af := pi.GetAddressFamily()

			prefixName := fmt.Sprintf("%s-%s", nodeName, strings.ReplaceAll(pi.String(), "/", "-"))
			prefixName = strings.ReplaceAll(prefixName, ":", "-")
			prefixAlloc, err := r.IpamClientProxy.Allocate(ctx, ipamv1alpha1.BuildIPAllocation(
				metav1.ObjectMeta{
					Name:      prefixName,
					Namespace: cr.Namespace,
				},
				ipamv1alpha1.IPAllocationSpec{
					Kind:            prefixKind,
					NetworkInstance: corev1.ObjectReference{Name: rtName, Namespace: cr.Namespace},
					AddressFamily:   &af,
					AllocationLabels: allocv1alpha1.AllocationLabels{
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								invv1alpha1.NephioNodeNameKey:  nodeName,
								allocv1alpha1.NephioPurposeKey: "node loopback address",
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: prefixSelectorLabels,
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
					IpPrefix: prefixAlloc.Status.Prefix,
				})

				polPsIpv6 := d.GetOrCreateRoutingPolicy().GetOrCreatePrefixSet("local-ipv6")
				polPsIpv6Key := ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_PrefixSet_Prefix_Key{
					IpPrefix:        pi.GetIPPrefix().String(),
					MaskLengthRange: "128..128",
				}
				polPsIpv6.Prefix = map[ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_PrefixSet_Prefix_Key]*ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_PrefixSet_Prefix{
					polPsIpv6Key: {
						IpPrefix:        ygot.String(pi.GetIPPrefix().String()),
						MaskLengthRange: ygot.String("128..128"),
					},
				}
				//rp.GetOrCreateStatement(20)
				if err := rp.AppendStatement(&ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_Policy_Statement{
					SequenceId: ygot.Uint32(20),
					Match: &ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_Policy_Statement_Match{
						PrefixSet: ygot.String("local-ipv6"),
					},
					Action: &ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_Policy_Statement_Action{
						PolicyResult: ygotsrl.E_SrlNokiaPolicyTypes_PolicyResultType(1),
					},
				}); err != nil {
					return err
				}
			} else {
				ipv4 := si.GetOrCreateIpv4()
				ipv4.AppendAddress(&ygotsrl.SrlNokiaInterfaces_Interface_Subinterface_Ipv4_Address{
					IpPrefix: prefixAlloc.Status.Prefix,
				})
				polPsIpv4 := d.GetOrCreateRoutingPolicy().GetOrCreatePrefixSet("local-ipv4")
				polPsIpv4Key := ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_PrefixSet_Prefix_Key{
					IpPrefix:        pi.GetIPPrefix().String(),
					MaskLengthRange: "32..32",
				}
				polPsIpv4.Prefix = map[ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_PrefixSet_Prefix_Key]*ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_PrefixSet_Prefix{
					polPsIpv4Key: {
						IpPrefix:        ygot.String(pi.GetIPPrefix().String()),
						MaskLengthRange: ygot.String("32..32"),
					},
				}
				//rp.GetOrCreateStatement(10)
				if err := rp.AppendStatement(&ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_Policy_Statement{
					SequenceId: ygot.Uint32(10),
					Match: &ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_Policy_Statement_Match{
						PrefixSet: ygot.String("local-ipv4"),
					},
					Action: &ygotsrl.SrlNokiaRoutingPolicy_RoutingPolicy_Policy_Statement_Action{
						PolicyResult: ygotsrl.E_SrlNokiaPolicyTypes_PolicyResultType(1),
					},
				}); err != nil {
					return err
				}
			}
		}

	}
	ni := d.GetOrCreateNetworkInstance(rtName)
	ni.GetOrCreateInterface(niItfceSubItfceName)

	bgp := ni.GetOrCreateProtocols().GetOrCreateBgp()
	bgp.AdminState = ygotsrl.SrlNokiaCommon_AdminState_enable
	bgp.AutonomousSystem = ygot.Uint32(65000)
	bgp.RouterId = ygot.String("1.1.1.1")
	bgp.EbgpDefaultPolicy = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_Bgp_EbgpDefaultPolicy{
		ExportRejectAll: ygot.Bool(false),
		ImportRejectAll: ygot.Bool(false),
	}
	bgp.Evpn = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_Bgp_Evpn{
		AdminState: ygotsrl.SrlNokiaCommon_AdminState_enable,
	}
	bgp.Ipv4Unicast = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_Bgp_Ipv4Unicast{
		AdminState: ygotsrl.SrlNokiaCommon_AdminState_enable,
		Multipath: &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_Bgp_Ipv4Unicast_Multipath{
			AllowMultipleAs: ygot.Bool(true),
			MaxPathsLevel_1: ygot.Uint32(64),
			MaxPathsLevel_2: ygot.Uint32(64),
		},
	}
	bgp.Ipv6Unicast = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_Bgp_Ipv6Unicast{
		AdminState: ygotsrl.SrlNokiaCommon_AdminState_enable,
		Multipath: &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_Bgp_Ipv6Unicast_Multipath{
			AllowMultipleAs: ygot.Bool(true),
			MaxPathsLevel_1: ygot.Uint32(64),
			MaxPathsLevel_2: ygot.Uint32(64),
		},
	}

	bgpOverlayGroup := bgp.GetOrCreateGroup("overlay")
	bgpOverlayGroup.AdminState = ygotsrl.SrlNokiaCommon_AdminState_enable
	bgpOverlayGroup.Evpn = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_Bgp_Group_Evpn{
		AdminState: ygotsrl.SrlNokiaCommon_AdminState_enable,
	}

	bgpUnderlayGroup := bgp.GetOrCreateGroup("underlay")
	bgpUnderlayGroup.AdminState = ygotsrl.SrlNokiaCommon_AdminState_enable
	bgpUnderlayGroup.NextHopSelf = ygot.Bool(true)
	bgpUnderlayGroup.ExportPolicy = ygot.String("export-local")
	bgpUnderlayGroup.Ipv4Unicast = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_Bgp_Group_Ipv4Unicast{
		AdminState: ygotsrl.SrlNokiaCommon_AdminState_enable,
	}
	bgpUnderlayGroup.Ipv6Unicast = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_Bgp_Group_Ipv6Unicast{
		AdminState: ygotsrl.SrlNokiaCommon_AdminState_enable,
	}
	bgpUnderlayGroup.Evpn = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_Bgp_Group_Evpn{
		AdminState: ygotsrl.SrlNokiaCommon_AdminState_enable,
	}

	d.GetOrCreateSystem().GetOrCreateNetworkInstance().GetOrCreateProtocols().GetOrCreateBgpVpn().GetOrCreateBgpInstance(1)
	d.GetOrCreateSystem().GetOrCreateNetworkInstance().GetOrCreateProtocols().GetOrCreateEvpn().GetOrCreateEthernetSegments().GetOrCreateBgpInstance(1)
	return nil
}
