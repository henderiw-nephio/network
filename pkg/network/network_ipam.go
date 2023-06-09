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

	infra2v1alpha1 "github.com/henderiw-nephio/network/apis/infra2/v1alpha1"
	infrav1alpha1 "github.com/nephio-project/api/infra/v1alpha1"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *network) addIPAMNetworkInstance(cr *infrav1alpha1.Network, rt infrav1alpha1.RoutingTable) client.Object {
	// create IPAM NetworkInstance
	o := r.ipam.ClaimIPAMDB(cr, rt.Name, rt.Prefixes)
	r.resources.AddNewResource(o)
	return o
}

func (r *network) getPoolPrefixes(ctx context.Context, cr *infrav1alpha1.Network, ifctx *ifceContext, prefixes []ipamv1alpha1.Prefix, labels map[string]string) (iputil.PrefixClaims, error) {
	prefixClaims := iputil.PrefixClaims{}
	for _, prefix := range prefixes {
		if prefix.GetPrefixKind() == ipamv1alpha1.PrefixKindPool {
			pi := iputil.NewPrefixInfo(netip.MustParsePrefix(prefix.Prefix))
			prefixLength := r.config.GetInterfacePrefixLength(infra2v1alpha1.InterfaceUsageKindPool, pi.IsIpv6())

			prefixClaimCtx := &ipamv1alpha1.PrefixClaimContext{
				PrefixClaimName: fmt.Sprintf("%s-%s", ipamv1alpha1.GetNameFromPrefix(pi.String(), ifctx.niName, "pool"), ifctx.selectorName),
				PrefixSelectorLabels: map[string]string{
					resourcev1alpha1.NephioNsnNameKey:      ipamv1alpha1.GetNameFromPrefix(pi.String(), ifctx.niName, ipamv1alpha1.NetworkInstancePrefixAggregate),
					resourcev1alpha1.NephioNsnNamespaceKey: cr.Namespace,
				},
				PrefixUserDefinedLabels: labels,
			}

			prefix, err := r.ipam.ClaimIPPrefix(ctx, cr, ifctx.niName, prefixClaimCtx.PrefixClaimName, ipamv1alpha1.PrefixKindPool, prefixLength, prefixClaimCtx.PrefixUserDefinedLabels, prefixClaimCtx.PrefixSelectorLabels)
			if err != nil {
				msg := fmt.Sprintf("cannot claim ip pool prefix for cr: %s, link: %s", cr.GetName(), prefixClaimCtx.PrefixClaimName)
				return nil, errors.Wrap(err, msg)
			}
			prefixClaims.AddPrefix(iputil.IsIPv6(pi.IsIpv6()), prefix)
		}
	}
	return prefixClaims, nil
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
			linkName := ifctx.linkName
			nodeName := ifctx.nodeName
			if ifctx.interfaceType == interfaceTypeIRB {
				linkName = "irb"
				nodeName = "gateway"
			} else {
				ifctx.bdName = ifctx.niName
			}
			newLabels := prefix.GetUserDefinedLabels()
			for k, v := range labels {
				newLabels[k] = v
			}

			prefixClaimCtx := prefix.GetPrefixClaimContext(ifctx.niName, ifctx.bdName, linkName, nodeName, cr.Namespace, newLabels, )

			_, err := r.ipam.ClaimIPPrefix(ctx, cr, ifctx.niName, prefixClaimCtx.PrefixClaimName, ipamv1alpha1.PrefixKindNetwork, prefixLength, prefixClaimCtx.PrefixUserDefinedLabels, prefixClaimCtx.PrefixSelectorLabels)
			if err != nil {
				msg := fmt.Sprintf("cannot claim ip link prefix for cr: %s, link: %s", cr.GetName(), prefixClaimCtx.PrefixClaimName)
				return nil, errors.Wrap(err, msg)
			}

			addressPrefix, err := r.ipam.ClaimIPAddress(ctx, cr, ifctx.niName, prefixClaimCtx.AddressClaimName, ipamv1alpha1.PrefixKindNetwork, prefixClaimCtx.AddressUserDefinedLabels, prefixClaimCtx.AddressSelectorLabels)
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

			addressPrefix, err := r.ipam.ClaimIPAddress(ctx, cr,
				ifctx.niName, addressClaimCxt.AddressClaimName, ipamv1alpha1.PrefixKindNetwork, addressClaimCxt.AddressUserDefinedLabels, addressClaimCxt.AddressSelectorLabels)
			if err != nil {
				msg := fmt.Sprintf("cannot claim ip adrress for cr: %s, ep: %s", cr.GetName(), addressClaimCxt.AddressClaimName)
				return nil, errors.Wrap(err, msg)
			}
			prefixClaims.AddPrefix(iputil.IsIPv6(pi.IsIpv6()), addressPrefix)
		}
	}
	return prefixClaims, nil
}
