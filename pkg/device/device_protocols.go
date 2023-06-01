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
	"github.com/openconfig/ygot/ygot"
	"github.com/srl-labs/ygotsrl/v22"
)

func (d *Device) AddRoutingProtocols(niName string) {
	ni := d.GetOrCreateNetworkInstance(niName)

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
}
