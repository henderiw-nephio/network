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

	infrav1alpha1 "github.com/henderiw-nephio/network/apis/infra2/v1alpha1"
	reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *network) addVlanDatabase(cr *infrav1alpha1.Network, dbIndexName string) client.Object {
	// create VLAN DataBase
	o := r.vlan.ClaimVLANDB(cr, dbIndexName)
	r.resources.AddNewResource(o)
	return o
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
