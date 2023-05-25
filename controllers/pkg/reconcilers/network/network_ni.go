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
	"k8s.io/utils/pointer"
)

// create a VLANDatabase (bdName + "-" + selectorName)
// create a BridgeDomain (bdName + "-" + selectorName)
// create BD Index (hash)
func (self *network) PopulateBridgeDomain(ctx context.Context, nodeName string, selectorName string, bdName string) (uint16, error) {
	// create VLAN DataBase
	o := vlanv1alpha1.BuildVLANDatabase(
		metav1.ObjectMeta{
			Name:            selectorName, // the vlan db is always the selectorName since the bd is physical and not virtual
			Namespace:       self.Namespace,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: self.APIVersion, Kind: self.Kind, Name: self.Name, UID: self.UID, Controller: pointer.Bool(true)}},
		},
		vlanv1alpha1.VLANDatabaseSpec{},
		vlanv1alpha1.VLANDatabaseStatus{},
	)
	self.resources[corev1.ObjectReference{APIVersion: o.APIVersion, Kind: o.Kind, Name: o.Name, Namespace: o.Namespace}] = o

	// create ni Intstance and interfaces
	//bdIndex := self.hash.Insert(bdName, "dummy", map[string]string{})
	ni := self.devices[nodeName].GetOrCreateNetworkInstance(bdName)
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

	// allocate vlanID -> vlanDatabase = selectorName, allocName = bdName or rtName
	return 10, nil
}

func (self *network) PopulateRoutingInstance(ctx context.Context, nodeName string, selectorName string, rtName string) (uint16, error) {

	// create VLAN DataBase
	o := vlanv1alpha1.BuildVLANDatabase(
		metav1.ObjectMeta{
			Name:            selectorName, // the vlan db is always the selectorName since the bd is physical and not virtual
			Namespace:       self.Namespace,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: self.APIVersion, Kind: self.Kind, Name: self.Name, UID: self.UID, Controller: pointer.Bool(true)}},
		},
		vlanv1alpha1.VLANDatabaseSpec{},
		vlanv1alpha1.VLANDatabaseStatus{},
	)
	self.resources[corev1.ObjectReference{APIVersion: o.GetResourceVersion(), Kind: o.GetObjectKind().GroupVersionKind().Kind, Name: o.GetName(), Namespace: o.GetNamespace()}] = o

	ni := self.devices[nodeName].GetOrCreateNetworkInstance(rtName)
	ni.Type = ygotsrl.SrlNokiaNetworkInstance_NiType_ip_vrf
	ni.IpForwarding = &ygotsrl.SrlNokiaNetworkInstance_NetworkInstance_IpForwarding{
		ReceiveIpv4Check: ygot.Bool(true),
		ReceiveIpv6Check: ygot.Bool(true),
	}
	/*
		rtIndex := self.hash.Insert(rtName + "-" + "rt", "dummy", map[string]string{})
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

	// allocate vlanID -> vlanDatabase = selectorName, allocName = bdName or rtName
	return 10, nil
}
