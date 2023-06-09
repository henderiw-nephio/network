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

	"github.com/henderiw-nephio/network/pkg/device"
	infrav1alpha1 "github.com/nephio-project/api/infra/v1alpha1"
	reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
)

func (r *network) AddNodeConfig(ctx context.Context, cr *infrav1alpha1.Network, ifctx *ifceContext, prefixes []ipamv1alpha1.Prefix) error {
	d := r.getDevice(ifctx.nodeName)
	pfxs, err := r.getLoopbackPrefixes(ctx, cr, ifctx, prefixes)
	if err != nil {
		return err
	}
	// adds the system interface
	d.AddRoutedInterface(ifctx.niName, device.SystemInterfaceName, 0, reqv1alpha1.AttachmentTypeNone, pfxs)

	// add the routing policy
	if err := d.AddRoutingPolicy(prefixes); err != nil {
		return err
	}
	// add the underlay and overlay protocol for the default routing instance
	d.AddRoutingProtocols(ifctx.niName)
	return nil
}
