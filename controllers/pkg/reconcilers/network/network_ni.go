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

	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/openconfig/ygot/ygot"
	"github.com/srl-labs/ygotsrl/v22"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// create a VLANDatabase (bdName + "-" + selectorName)
// create a BridgeDomain (bdName + "-" + selectorName)
// create BD Index (hash)
func (r *network) PopulateBridgeDomain(ctx context.Context, nodeName string, selectorName string, bdName string) (uint16, error) {
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = new(ygotsrl.Device)
	}

	// create ni Intstance and interfaces
	//bdIndex := r.hash.Insert(bdName, "dummy", map[string]string{})
	ni := r.devices[nodeName].GetOrCreateNetworkInstance(bdName)
	ni.Type = ygotsrl.SrlNokiaNetworkInstance_NiType_mac_vrf
	ni.BridgeTable = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_BridgeTable{
		MacLearning: &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_BridgeTable_MacLearning{
			AdminState: ygotsrl.SrlNokiaCommon_AdminState_enable,
		},
	}
	/*
		bgpEvpn := ni.GetOrCreateProtocols().GetOrCreateBgpEvpn()
		bgpEvpnBgpInstance := bgpEvpn.GetOrCreateBgpInstance(1)
		bgpEvpnBgpInstance.AdminState = ygotsrl.SrlNokiaCommon_AdminState_enable
		bgpEvpnBgpInstance.Evi = ygot.Uint32(bdIndex)
		bgpEvpnBgpInstance.Ecmp = ygot.Uint8(2)
		bgpEvpnBgpInstance.VxlanInterface = ygot.String(strings.Join([]string{"vxlan0", strconv.Itoa(int(bdIndex))}, "."))

		bgpVpn := ni.GetOrCreateProtocols().GetOrCreateBgpVpn()
		bgpVpnBgpInstance := bgpVpn.GetOrCreateBgpInstance(1)
		bgpVpnBgpInstance.RouteTarget = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_BgpVpn_BgpInstance_RouteTarget{
			ImportRt: ygot.String(strings.Join([]string{"target", "65555", strconv.Itoa(int(bdIndex))}, ":")),
			ExportRt: ygot.String(strings.Join([]string{"target", "65555", strconv.Itoa(int(bdIndex))}, ":")),
		}
	*/
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
		return 0, err
	}

	// allocate vlanID -> vlanDatabase = selectorName, allocName = bdName or rtName
	return *vlanAlloc.Status.VLANID, nil
}

func (r *network) PopulateRoutingInstance(ctx context.Context, nodeName string, selectorName string, rtName string) (uint16, error) {
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = new(ygotsrl.Device)
	}

	ni := r.devices[nodeName].GetOrCreateNetworkInstance(rtName)
	ni.Type = ygotsrl.SrlNokiaNetworkInstance_NiType_ip_vrf
	ni.IpForwarding = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_IpForwarding{
		ReceiveIpv4Check: ygot.Bool(true),
		ReceiveIpv6Check: ygot.Bool(true),
	}
	/*
		rtIndex := r.hash.Insert(rtName + "-" + "rt", "dummy", map[string]string{})
		bgpEvpn := ni.GetOrCreateProtocols().GetOrCreateBgpEvpn()
		bgpEvpnBgpInstance := bgpEvpn.GetOrCreateBgpInstance(1)
		bgpEvpnBgpInstance.AdminState = ygotsrl.SrlNokiaCommon_AdminState_enable
		bgpEvpnBgpInstance.Evi = ygot.Uint32(rtIndex)
		bgpEvpnBgpInstance.Ecmp = ygot.Uint8(2)
		bgpEvpnBgpInstance.VxlanInterface = ygot.String(strings.Join([]string{"vxlan0", strconv.Itoa(int(rtIndex))}, "."))

		bgpVpn := ni.GetOrCreateProtocols().GetOrCreateBgpVpn()
		bgpVpnBgpInstance := bgpVpn.GetOrCreateBgpInstance(1)
		bgpVpnBgpInstance.RouteTarget = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_Protocols_BgpVpn_BgpInstance_RouteTarget{
			ImportRt: ygot.String(strings.Join([]string{"target", "65555", strconv.Itoa(int(rtIndex))}, ":")),
			ExportRt: ygot.String(strings.Join([]string{"target", "65555", strconv.Itoa(int(rtIndex))}, ":")),
		}
	*/

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
		return 0, err
	}
	// allocate vlanID -> vlanDatabase = selectorName, allocName = bdName or rtName
	return *vlanAlloc.Status.VLANID, nil
}
