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

	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
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
	ClaimIPPrefix(ctx context.Context, cr client.Object, dbIndexName, claimName string, prefixKind ipamv1alpha1.PrefixKind, prefixLength uint8, udl, sel map[string]string) (*string, error)
	ClaimIPAddress(ctx context.Context, cr client.Object, dbIndexName, claimName string, prefixKind ipamv1alpha1.PrefixKind, udl, sel map[string]string) (*string, error)
}

func NewIPAM(c clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPClaim]) IPAM {
	return &ipam{c}
}

type ipam struct {
	clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPClaim]
}

// claimIPAMDB return a KRM IPAM network instance as client.Object
// the dbIndexName is the networkInstance name
func (r *ipam) ClaimIPAMDB(cr client.Object, dbIndexName string, prefixes []ipamv1alpha1.Prefix) *ipamv1alpha1.NetworkInstance {
	return ipamv1alpha1.BuildNetworkInstance(
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
		}, ipamv1alpha1.NetworkInstanceSpec{
			Prefixes: prefixes,
		}, ipamv1alpha1.NetworkInstanceStatus{})
}

func (r *ipam) ClaimIPPrefix(ctx context.Context, cr client.Object, dbIndexName, claimName string, prefixKind ipamv1alpha1.PrefixKind, prefixLength uint8, udl, sel map[string]string) (*string, error) {
	prefix, err := r.Claim(ctx, ipamv1alpha1.BuildIPClaim(
		metav1.ObjectMeta{
			Name:      claimName,
			Namespace: cr.GetNamespace(),
		},
		ipamv1alpha1.IPClaimSpec{
			Kind:            prefixKind,
			NetworkInstance: corev1.ObjectReference{Name: dbIndexName, Namespace: cr.GetNamespace()},
			//AddressFamily:   &af, -> automatically calculated + we allocate based on nsn
			PrefixLength: util.PointerUint8(int(prefixLength)),
			CreatePrefix: pointer.Bool(true),
			ClaimLabels: resourcev1alpha1.ClaimLabels{
				UserDefinedLabels: resourcev1alpha1.UserDefinedLabels{
					Labels: udl,
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: sel,
				},
			},
		},
		ipamv1alpha1.IPClaimStatus{},
	), nil)
	if err != nil {
		return nil, err
	}
	return prefix.Status.Prefix, nil
}

func (r *ipam) ClaimIPAddress(ctx context.Context, cr client.Object, dbIndexName, claimName string, prefixKind ipamv1alpha1.PrefixKind, udl, sel map[string]string) (*string, error) {
	address, err := r.Claim(ctx, ipamv1alpha1.BuildIPClaim(
		metav1.ObjectMeta{
			Name:      claimName,
			Namespace: cr.GetNamespace(),
		},
		ipamv1alpha1.IPClaimSpec{
			Kind:            prefixKind,
			NetworkInstance: corev1.ObjectReference{Name: dbIndexName, Namespace: cr.GetNamespace()},
			//AddressFamily:   &af,
			ClaimLabels: resourcev1alpha1.ClaimLabels{
				UserDefinedLabels: resourcev1alpha1.UserDefinedLabels{
					Labels: udl,
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: sel,
				},
			},
		},
		ipamv1alpha1.IPClaimStatus{},
	), nil)
	if err != nil {
		return nil, err
	}
	return address.Status.Prefix, nil
}
