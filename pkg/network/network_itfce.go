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

	infra2v1alpha1 "github.com/henderiw-nephio/network/apis/infra2/v1alpha1"
	infrav1alpha1 "github.com/nephio-project/api/infra/v1alpha1"
	"github.com/henderiw-nephio/network/pkg/device"
	reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
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
	interfaceKind  infra2v1alpha1.InterfaceUsageKind
	linkName       string // ep.GetLinkName(int(vlanId))
	vlanDBIndex    string // name of node/itfce or selectorName (e.g. cluster)
	bdName         string // only used in routed IRB - used to calculate the index matching the bdName
	niName         string // generic name for network instance can be rt or bd
	attachmentType reqv1alpha1.AttachmentType
	selectorName   string // used for pool allocations
}

func (r *network) AddBridgeInterface(ctx context.Context, cr *infrav1alpha1.Network, ifctx *ifceContext) error {
	// allocate a vlanID if the attachment type is VLAN
	vlanId, err := r.getVLANID(ctx, cr, ifctx)
	if err != nil {
		return err
	}

	d := r.getDevice(ifctx.nodeName)
	d.AddBridgedInterface(ifctx.niName, ifctx.ifName, int(vlanId), ifctx.attachmentType)
	return nil
}

func (r *network) AddBridgeIRBInterface(ctx context.Context, cr *infrav1alpha1.Network, ifctx *ifceContext) error {
	index := r.hash.Insert(ifctx.bdName, "dummy", map[string]string{})
	d := r.getDevice(ifctx.nodeName)
	d.AddBridgedInterface(ifctx.bdName, device.IRBInterfaceName, int(index), reqv1alpha1.AttachmentTypeNone)
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

func (r *network) AddRoutedIRBInterface(ctx context.Context, cr *infrav1alpha1.Network, ifctx *ifceContext, prefixes []ipamv1alpha1.Prefix, labels map[string]string) error {
	index := r.hash.Insert(ifctx.bdName, "dummy", map[string]string{})

	// pools are now allocated per cluster to make static routes easier
	_, err := r.getPoolPrefixes(ctx, cr, ifctx, prefixes, labels)
	if err != nil {
		return err
	}

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
