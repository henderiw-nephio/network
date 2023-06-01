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

	infrav1alpha1 "github.com/henderiw-nephio/network/apis/infra/v1alpha1"
	"github.com/henderiw-nephio/network/pkg/device"
	reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/pkg/errors"
	"github.com/srl-labs/ygotsrl/v22"
)

// special case - in irb routed
// prefixName is based on bdName
// NetworkInstance is based on rtName

type interfaceType string

const (
	interfaceTypeIRB     interfaceType = "irb"
	interfaceTypeRegular interfaceType = "regular"
)

type ifceContext struct {
	nodeName       string
	ifName         string
	interfaceType  interfaceType // "irb or "regular"
	interfaceKind  infrav1alpha1.InterfaceUsageKind
	linkName       string // ep.GetLinkName(int(vlanId))
	vlanDBIndex    string // name of node/itfce or selectorName (e.g. cluster)
	bdName         string // only used in routed IRB - used to calculate the index matching the bdName
	niName         string // generic name for network instance can be rt or bd
	attachmentType reqv1alpha1.AttachmentType
}

func (r *network) getDevice(nodeName string) *device.Device {
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = new(ygotsrl.Device)
	}
	return &device.Device{Device: r.devices[nodeName]}
}

func (r *network) getVLANID(ctx context.Context, cr *infrav1alpha1.Network, ifCtxt *ifceContext) (uint16, error) {
	vlanId := uint16(0)
	if ifCtxt.attachmentType == reqv1alpha1.AttachmentTypeVLAN {
		vlanID, err := r.vlan.ClaimVLANID(ctx, cr, ifCtxt.vlanDBIndex, ifCtxt.niName)
		if err != nil {
			msg := fmt.Sprintf("cannot claim vlan for cr: %s, niName: %s", cr.GetName(), ifCtxt.niName)
			return 0, errors.Wrap(err, msg)
		}
		vlanId = *vlanID
	}
	return vlanId, nil
}

func (r *network) getLinkPrefixes(ctx context.Context, cr *infrav1alpha1.Network, ifctx *ifceContext, prefixes []ipamv1alpha1.Prefix, labels map[string]string) (iputil.PrefixClaims, error) {
	prefixClaims := iputil.PrefixClaims{}
	for _, prefix := range prefixes {
		if prefix.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
			pi := iputil.NewPrefixInfo(netip.MustParsePrefix(prefix.Prefix))
			prefixLength := r.config.GetInterfacePrefixLength(ifctx.interfaceKind, pi.IsIpv6())

			// special case - in irb routed
			// prefixName is based on bdName
			// NetworkInstance is based on rtName
			niName := ifctx.niName
			linkName := ifctx.linkName
			nodeName := ifctx.nodeName
			if ifctx.interfaceType == interfaceTypeIRB {
				niName = ifctx.bdName
				linkName = "irb"
				nodeName = "gateway"
			}
			prefixClaimCtx := prefix.GetPrefixClaimContext(niName, linkName, nodeName, cr.Namespace, labels)

			_, err := r.ipam.ClaimIPPrefix(ctx, cr, ifctx.niName, prefixClaimCtx.PrefixClaimName, prefixLength, prefixClaimCtx.PrefixUserDefinedLabels, prefixClaimCtx.PrefixSelectorLabels)
			if err != nil {
				msg := fmt.Sprintf("cannot claim ip prefix for cr: %s, link: %s", cr.GetName(), prefixClaimCtx.PrefixClaimName)
				return nil, errors.Wrap(err, msg)
			}

			addressPrefix, err := r.ipam.ClaimIPAddress(ctx, cr, ifctx.niName, prefixClaimCtx.AddressClaimName, prefixClaimCtx.AddressUserDefinedLabels, prefixClaimCtx.AddressSelectorLabels)
			if err != nil {
				msg := fmt.Sprintf("cannot claim ip adrress for cr: %s, ep: %s", cr.GetName(), prefixClaimCtx.AddressClaimName)
				return nil, errors.Wrap(err, msg)
			}
			prefixClaims.AddPrefix(iputil.IsIPv6(pi.IsIpv6()), addressPrefix)
		}
	}
	return prefixClaims, nil
}

func (r *network) getLoopbackPrefixes(ctx context.Context, cr *infrav1alpha1.Network, ifctx *ifceContext, prefixes []ipamv1alpha1.Prefix) (iputil.PrefixClaims, error) {
	prefixClaims := iputil.PrefixClaims{}
	for _, prefix := range prefixes {
		if prefix.GetPrefixKind() == ipamv1alpha1.PrefixKindLoopback {
			pi := iputil.NewPrefixInfo(netip.MustParsePrefix(prefix.Prefix))
			addressClaimCxt := prefix.GetAddressClaimContext(ifctx.niName, ifctx.nodeName, cr.Namespace, map[string]string{})

			addressPrefix, err := r.ipam.ClaimIPAddress(ctx, cr, ifctx.niName, addressClaimCxt.AddressClaimName, addressClaimCxt.AddressUserDefinedLabels, addressClaimCxt.AddressSelectorLabels)
			if err != nil {
				msg := fmt.Sprintf("cannot claim ip adrress for cr: %s, ep: %s", cr.GetName(), addressClaimCxt.AddressClaimName)
				return nil, errors.Wrap(err, msg)
			}
			prefixClaims.AddPrefix(iputil.IsIPv6(pi.IsIpv6()), addressPrefix)
		}
	}
	return prefixClaims, nil
}

func (r *network) AddBridgeInterface(ctx context.Context, cr *infrav1alpha1.Network, ifCtxt *ifceContext) error {
	// allocate a vlanID if the attachment type is VLAN
	vlanId, err := r.getVLANID(ctx, cr, ifCtxt)
	if err != nil {
		return err
	}

	d := r.getDevice(ifCtxt.nodeName)
	d.AddBridgedInterface(ifCtxt.niName, ifCtxt.ifName, int(vlanId), ifCtxt.attachmentType)
	return nil
}

func (r *network) AddIRBBridgeInterface(ctx context.Context, cr *infrav1alpha1.Network, ifCtxt *ifceContext) error {
	index := r.hash.Insert(ifCtxt.bdName, "dummy", map[string]string{})
	d := r.getDevice(ifCtxt.nodeName)
	d.AddBridgedInterface(ifCtxt.bdName, device.IRBInterfaceName, int(index), reqv1alpha1.AttachmentTypeNone)
	return nil
}

func (r *network) AddRoutedInterface(ctx context.Context, cr *infrav1alpha1.Network, ifctx *ifceContext, prefixes []ipamv1alpha1.Prefix, labels map[string]string) error {
	// allocate a vlanID if the attachment type is VLAN
	vlanId, err := r.getVLANID(ctx, cr, ifctx)
	if err != nil {
		return err
	}
	// we update the linkname and add the index to get a unique name per link
	ifctx.linkName = fmt.Sprintf("%s-%d", ifctx.linkName, vlanId)
	pfxs, err := r.getLinkPrefixes(ctx, cr, ifctx, prefixes, labels)
	if err != nil {
		return err
	}
	d := r.getDevice(ifctx.nodeName)
	d.AddRoutedInterface(ifctx.niName, ifctx.ifName, int(vlanId), ifctx.attachmentType, pfxs)
	return nil
}

func (r *network) AddIRBRoutedInterface(ctx context.Context, cr *infrav1alpha1.Network, ifctx *ifceContext, prefixes []ipamv1alpha1.Prefix, labels map[string]string) error {
	index := r.hash.Insert(ifctx.bdName, "dummy", map[string]string{})

	pfxs, err := r.getLinkPrefixes(ctx, cr, ifctx, prefixes, labels)
	if err != nil {
		return err
	}

	d := r.getDevice(ifctx.nodeName)
	d.AddRoutedInterface(ifctx.niName, device.IRBInterfaceName, int(index), reqv1alpha1.AttachmentTypeNone, pfxs)
	return nil
}

func (r *network) AddLoopbackInterface(ctx context.Context, cr *infrav1alpha1.Network, ifctx *ifceContext, prefixes []ipamv1alpha1.Prefix) error {
	index := 0

	pfxs, err := r.getLoopbackPrefixes(ctx, cr, ifctx, prefixes)
	if err != nil {
		return err
	}
	d := r.getDevice(ifctx.nodeName)
	d.AddRoutedInterface(ifctx.niName, device.SystemInterfaceName, int(index), reqv1alpha1.AttachmentTypeNone, pfxs)
	return nil
}
