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

package ipam

import (
	"context"

	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/nokia/k8s-ipam/pkg/utils/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IPAM interface {
	ClaimIPAMDB(cr client.Object, dbIndexName string, prefixes []ipamv1alpha1.Prefix) *ipamv1alpha1.NetworkInstance
	ClaimIPPrefix(ctx context.Context, cr client.Object, dbIndexName, claimName string, prefixLength uint8, udl, sel map[string]string) (*string, error)
	ClaimIPAddress(ctx context.Context, cr client.Object, dbIndexName, claimName string, udl, sel map[string]string) (*string, error)
}

func NewIPAM(c clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation]) IPAM {
	return &ipam{c}
}

type ipam struct {
	clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation]
}

// claimIPAMDB return a KRM IPAM network instance as client.Object
// the dbIndexName is the networkInstance name
func (r *ipam) ClaimIPAMDB(cr client.Object, dbIndexName string, prefixes []ipamv1alpha1.Prefix) *ipamv1alpha1.NetworkInstance {
	return ipamv1alpha1.BuildNetworkInstance(
		metav1.ObjectMeta{
			Name:      dbIndexName,
			Namespace: cr.GetNamespace(),
			Labels:    allocv1alpha1.GetOwnerLabelsFromCR(cr),
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
		}, ipamv1alpha1.NetworkInstanceSpec{
			Prefixes: prefixes,
		}, ipamv1alpha1.NetworkInstanceStatus{})
}

func (r *ipam) ClaimIPPrefix(ctx context.Context, cr client.Object, dbIndexName, claimName string, prefixLength uint8, udl, sel map[string]string) (*string, error) {
	prefix, err := r.Allocate(ctx, ipamv1alpha1.BuildIPAllocation(
		metav1.ObjectMeta{
			Name:      claimName,
			Namespace: cr.GetNamespace(),
		},
		ipamv1alpha1.IPAllocationSpec{
			Kind:            ipamv1alpha1.PrefixKindNetwork,
			NetworkInstance: corev1.ObjectReference{Name: dbIndexName, Namespace: cr.GetNamespace()},
			//AddressFamily:   &af,
			PrefixLength: util.PointerUint8(int(prefixLength)),
			CreatePrefix: pointer.Bool(true),
			AllocationLabels: allocv1alpha1.AllocationLabels{
				UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
					Labels: udl,
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: sel,
				},
			},
		},
		ipamv1alpha1.IPAllocationStatus{},
	), nil)
	if err != nil {
		return nil, err
	}
	return prefix.Status.Prefix, nil
}

func (r *ipam) ClaimIPAddress(ctx context.Context, cr client.Object, dbIndexName, claimName string, udl, sel map[string]string) (*string, error) {
	address, err := r.Allocate(ctx, ipamv1alpha1.BuildIPAllocation(
		metav1.ObjectMeta{
			Name:      claimName,
			Namespace: cr.GetNamespace(),
		},
		ipamv1alpha1.IPAllocationSpec{
			Kind:            ipamv1alpha1.PrefixKindNetwork,
			NetworkInstance: corev1.ObjectReference{Name: dbIndexName, Namespace: cr.GetNamespace()},
			//AddressFamily:   &af,
			AllocationLabels: allocv1alpha1.AllocationLabels{
				UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
					Labels: udl,
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: sel,
				},
			},
		},
		ipamv1alpha1.IPAllocationStatus{},
	), nil)
	if err != nil {
		return nil, err
	}
	return address.Status.Prefix, nil
}
