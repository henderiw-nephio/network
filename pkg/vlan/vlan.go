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

package vlan

import (
	"context"

	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type VLAN interface {
	ClaimVLANDB(cr client.Object, dbIndexName string) *vlanv1alpha1.VLANIndex
	ClaimVLANID(ctx context.Context, cr client.Object, dbIndexName, claimName string) (*uint16, error)
}

func NewVLAN(c clientproxy.Proxy[*vlanv1alpha1.VLANIndex, *vlanv1alpha1.VLANClaim]) VLAN {
	return &vlan{c}
}

type vlan struct {
	clientproxy.Proxy[*vlanv1alpha1.VLANIndex, *vlanv1alpha1.VLANClaim]
}

func (r *vlan) ClaimVLANDB(cr client.Object, dbIndexName string) *vlanv1alpha1.VLANIndex {
	return vlanv1alpha1.BuildVLANIndex(
		metav1.ObjectMeta{
			Name:      dbIndexName,
			Namespace: cr.GetNamespace(),
			Labels:    resourcev1alpha1.GetOwnerLabelsFromCR(cr),
			OwnerReferences: []metav1.OwnerReference{
				{

					APIVersion: schema.GroupVersion{
						Group:   cr.GetObjectKind().GroupVersionKind().Group,
						Version: cr.GetObjectKind().GroupVersionKind().Version,
					}.String(),
					Kind:       cr.GetObjectKind().GroupVersionKind().Kind,
					Name:       cr.GetName(),
					UID:        cr.GetUID(),
					Controller: pointer.Bool(true),
				},
			},
		},
		vlanv1alpha1.VLANIndexSpec{},
		vlanv1alpha1.VLANIndexStatus{},
	)
}

func (r *vlan) ClaimVLANID(ctx context.Context, cr client.Object, dbIndexName, claimName string) (*uint16, error) {
	vlanAlloc, err := r.Claim(ctx, vlanv1alpha1.BuildVLANClaim(
		metav1.ObjectMeta{
			Name:      claimName,
			Namespace: cr.GetNamespace(),
		},
		vlanv1alpha1.VLANClaimSpec{
			VLANIndex: corev1.ObjectReference{Name: dbIndexName, Namespace: cr.GetNamespace()},
		},
		vlanv1alpha1.VLANClaimStatus{},
	), nil)
	if err != nil {
		return nil, err
	}
	return vlanAlloc.Status.VLANID, nil
}
