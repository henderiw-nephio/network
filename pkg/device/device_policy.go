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
	"net/netip"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/openconfig/ygot/ygot"
	"github.com/srl-labs/ygotsrl/v22"
)

func (d Device) AddRoutingPolicy(prefixes []ipamv1alpha1.Prefix) error {
	// create a routing policy
	rp := d.GetOrCreateRoutingPolicy().GetOrCreatePolicy("export-local")
	rp.Name = ygot.String("export-local")

	for _, prefix := range prefixes {
		pi := iputil.NewPrefixInfo(netip.MustParsePrefix(prefix.Prefix))

		if prefix.GetPrefixKind() == ipamv1alpha1.PrefixKindLoopback {
			if pi.IsIpv6() {
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
	return nil
}
