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
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/hansthienpondt/nipam/pkg/table"
	configv1alpha1 "github.com/henderiw-nephio/network/apis/config/v1alpha1"
	infrav1alpha1 "github.com/henderiw-nephio/network/apis/infra2/v1alpha1"
	"github.com/henderiw-nephio/network/pkg/endpoints"
	ipamcl "github.com/henderiw-nephio/network/pkg/ipam"
	"github.com/henderiw-nephio/network/pkg/nodes"
	"github.com/henderiw-nephio/network/pkg/resources"
	vlancl "github.com/henderiw-nephio/network/pkg/vlan"

	//reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	reqv1alpha1 "github.com/nephio-project/api/nf_requirements/v1alpha1"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	ipambe "github.com/nokia/k8s-ipam/pkg/backend/ipam"
	vlanbe "github.com/nokia/k8s-ipam/pkg/backend/vlan"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy/ipam"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy/vlan"
	"github.com/openconfig/ygot/ygot"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
)

var testEndpointsSingleNode = &endpoints.Endpoints{
	EndpointList: &invv1alpha1.EndpointList{
		Items: []invv1alpha1.Endpoint{
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: invv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
					Kind:       invv1alpha1.EndpointKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "srl-e1-1",
					Namespace: "default",
					Labels: map[string]string{
						invv1alpha1.NephioTopologyKey:      "nephio",
						invv1alpha1.NephioProviderKey:      "srl.nokia.com",
						invv1alpha1.NephioNodeNameKey:      "srl",
						invv1alpha1.NephioInterfaceNameKey: "e1-1",
						invv1alpha1.NephioClusterNameKey:   "cluster01",
					},
				},
				Spec: invv1alpha1.EndpointSpec{
					EndpointProperties: invv1alpha1.EndpointProperties{
						InterfaceName: "e1-1",
						NodeName:      "srl",
					},
					Provider: invv1alpha1.Provider{
						Provider: "srl.nokia.com",
					},
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: invv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
					Kind:       invv1alpha1.EndpointKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "srl-e1-2",
					Namespace: "default",
					Labels: map[string]string{
						invv1alpha1.NephioTopologyKey:      "nephio",
						invv1alpha1.NephioProviderKey:      "srl.nokia.com",
						invv1alpha1.NephioNodeNameKey:      "srl",
						invv1alpha1.NephioInterfaceNameKey: "e1-2",
						invv1alpha1.NephioClusterNameKey:   "cluster02",
					},
				},
				Spec: invv1alpha1.EndpointSpec{
					EndpointProperties: invv1alpha1.EndpointProperties{
						InterfaceName: "e1-2",
						NodeName:      "srl",
					},
					Provider: invv1alpha1.Provider{
						Provider: "srl.nokia.com",
					},
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: invv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
					Kind:       invv1alpha1.EndpointKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "srl-e1-3",
					Namespace: "default",
					Labels: map[string]string{
						invv1alpha1.NephioTopologyKey:      "nephio",
						invv1alpha1.NephioProviderKey:      "srl.nokia.com",
						invv1alpha1.NephioNodeNameKey:      "srl",
						invv1alpha1.NephioInterfaceNameKey: "e1-3",
						invv1alpha1.NephioClusterNameKey:   "cluster03",
					},
				},
				Spec: invv1alpha1.EndpointSpec{
					EndpointProperties: invv1alpha1.EndpointProperties{
						InterfaceName: "e1-3",
						NodeName:      "srl",
					},
					Provider: invv1alpha1.Provider{
						Provider: "srl.nokia.com",
					},
				},
			},
		},
	},
}

var singleNode = &nodes.Nodes{
	NodeList: &invv1alpha1.NodeList{
		Items: []invv1alpha1.Node{
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: invv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
					Kind:       invv1alpha1.NodeKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "srl",
					Namespace: "default",
					Labels: map[string]string{
						invv1alpha1.NephioTopologyKey: "nephio",
						invv1alpha1.NephioProviderKey: "srl.nokia.com",
						invv1alpha1.NephioNodeNameKey: "srl",
					},
				},
				Spec: invv1alpha1.NodeSpec{
					Provider: "srl.nokia.com",
				},
			},
		},
	},
}

var testEndpointsMultiNode = &endpoints.Endpoints{
	EndpointList: &invv1alpha1.EndpointList{
		Items: []invv1alpha1.Endpoint{
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: invv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
					Kind:       invv1alpha1.EndpointKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "srl1-e1-1",
					Namespace: "default",
					Labels: map[string]string{
						invv1alpha1.NephioTopologyKey:      "nephio",
						invv1alpha1.NephioProviderKey:      "srl.nokia.com",
						invv1alpha1.NephioNodeNameKey:      "srl1",
						invv1alpha1.NephioInterfaceNameKey: "e1-1",
						invv1alpha1.NephioClusterNameKey:   "cluster01",
					},
				},
				Spec: invv1alpha1.EndpointSpec{
					EndpointProperties: invv1alpha1.EndpointProperties{
						InterfaceName: "e1-1",
						NodeName:      "srl1",
					},
					Provider: invv1alpha1.Provider{
						Provider: "srl.nokia.com",
					},
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: invv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
					Kind:       invv1alpha1.EndpointKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "srl2-e1-2",
					Namespace: "default",
					Labels: map[string]string{
						invv1alpha1.NephioTopologyKey:      "nephio",
						invv1alpha1.NephioProviderKey:      "srl.nokia.com",
						invv1alpha1.NephioNodeNameKey:      "srl2",
						invv1alpha1.NephioInterfaceNameKey: "e1-2",
						invv1alpha1.NephioClusterNameKey:   "cluster02",
					},
				},
				Spec: invv1alpha1.EndpointSpec{
					EndpointProperties: invv1alpha1.EndpointProperties{
						InterfaceName: "e1-2",
						NodeName:      "srl2",
					},
					Provider: invv1alpha1.Provider{
						Provider: "srl.nokia.com",
					},
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: invv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
					Kind:       invv1alpha1.EndpointKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "srl3-e1-3",
					Namespace: "default",
					Labels: map[string]string{
						invv1alpha1.NephioTopologyKey:      "nephio",
						invv1alpha1.NephioProviderKey:      "srl.nokia.com",
						invv1alpha1.NephioNodeNameKey:      "srl3",
						invv1alpha1.NephioInterfaceNameKey: "e1-3",
						invv1alpha1.NephioClusterNameKey:   "cluster03",
					},
				},
				Spec: invv1alpha1.EndpointSpec{
					EndpointProperties: invv1alpha1.EndpointProperties{
						InterfaceName: "e1-3",
						NodeName:      "srl3",
					},
					Provider: invv1alpha1.Provider{
						Provider: "srl.nokia.com",
					},
				},
			},
		},
	},
}

var multiNode = &nodes.Nodes{
	NodeList: &invv1alpha1.NodeList{
		Items: []invv1alpha1.Node{
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: invv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
					Kind:       invv1alpha1.NodeKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "srl1",
					Namespace: "default",
					Labels: map[string]string{
						invv1alpha1.NephioTopologyKey: "nephio",
						invv1alpha1.NephioProviderKey: "srl.nokia.com",
						invv1alpha1.NephioNodeNameKey: "srl1",
					},
				},
				Spec: invv1alpha1.NodeSpec{
					Provider: "srl.nokia.com",
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: invv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
					Kind:       invv1alpha1.NodeKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "srl2",
					Namespace: "default",
					Labels: map[string]string{
						invv1alpha1.NephioTopologyKey: "nephio",
						invv1alpha1.NephioProviderKey: "srl.nokia.com",
						invv1alpha1.NephioNodeNameKey: "srl2",
					},
				},
				Spec: invv1alpha1.NodeSpec{
					Provider: "srl.nokia.com",
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: invv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
					Kind:       invv1alpha1.NodeKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "srl3",
					Namespace: "default",
					Labels: map[string]string{
						invv1alpha1.NephioTopologyKey: "nephio",
						invv1alpha1.NephioProviderKey: "srl.nokia.com",
						invv1alpha1.NephioNodeNameKey: "srl3",
					},
				},
				Spec: invv1alpha1.NodeSpec{
					Provider: "srl.nokia.com",
				},
			},
		},
	},
}

var testDefaultCR = &infrav1alpha1.Network{
	TypeMeta: metav1.TypeMeta{
		APIVersion: infrav1alpha1.SchemeBuilder.GroupVersion.Identifier(),
		Kind:       infrav1alpha1.NetworkKind,
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "internet",
		Namespace: "default",
	},
	Spec: infrav1alpha1.NetworkSpec{
		Topology: "nephio",
		RoutingTables: []infrav1alpha1.RoutingTable{
			{
				Name: "default",
				Prefixes: []ipamv1alpha1.Prefix{
					{
						Prefix: "2000::/32",
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								allocv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindNetwork),
							},
						},
					},
					{
						Prefix: "10.0.0.0/16",
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								allocv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindNetwork),
							},
						},
					},
					{
						Prefix: "192.0.0.0/16",
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								allocv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindLoopback),
							},
						},
					},
					{
						Prefix: "1000::/64",
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								allocv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindLoopback),
							},
						},
					},
				},
				Interfaces: []infrav1alpha1.Interface{
					{
						Kind:             infrav1alpha1.InterfaceKindBridgeDomain,
						BridgeDomainName: pointer.String("internet"),
					},
				},
			},
		},
	},
}

var testInternetCR = &infrav1alpha1.Network{
	TypeMeta: metav1.TypeMeta{
		APIVersion: infrav1alpha1.SchemeBuilder.GroupVersion.Identifier(),
		Kind:       infrav1alpha1.NetworkKind,
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "internet",
		Namespace: "default",
	},
	Spec: infrav1alpha1.NetworkSpec{
		Topology: "nephio",
		BridgeDomains: []infrav1alpha1.BridgeDomain{
			{
				Name: "internet",
				Interfaces: []infrav1alpha1.Interface{
					{
						Kind: infrav1alpha1.InterfaceKindInterface,
						Selector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      invv1alpha1.NephioClusterNameKey,
									Operator: "Exists",
								},
							},
						},
						AttachmentType: reqv1alpha1.AttachmentTypeVLAN,
					},
				},
			},
		},
		RoutingTables: []infrav1alpha1.RoutingTable{
			{
				Name: "internet",
				Prefixes: []ipamv1alpha1.Prefix{
					{
						Prefix: "172::/32",
					},
					{
						Prefix: "172.0.0.0/16",
					},
					{
						Prefix: "10.0.0.0/8",
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								allocv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindPool),
								allocv1alpha1.NephioPurposeKey:    "pool",
							},
						},
					},
					{
						Prefix: "1000::/32",
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								allocv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindPool),
								allocv1alpha1.NephioPurposeKey:    "pool",
							},
						},
					},
				},
				Interfaces: []infrav1alpha1.Interface{
					{
						Kind:             infrav1alpha1.InterfaceKindBridgeDomain,
						BridgeDomainName: pointer.String("internet"),
					},
				},
			},
		},
	},
}

var vlanDBs = &vlanv1alpha1.VLANDatabaseList{
	Items: []vlanv1alpha1.VLANDatabase{
		{
			TypeMeta: metav1.TypeMeta{
				APIVersion: vlanv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
				Kind:       vlanv1alpha1.VLANDatabaseKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster01",
				Namespace: "default",
			},
			Spec: vlanv1alpha1.VLANDatabaseSpec{
				Kind: vlanv1alpha1.VLANDBKindESG,
			},
		},
		{
			TypeMeta: metav1.TypeMeta{
				APIVersion: vlanv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
				Kind:       vlanv1alpha1.VLANDatabaseKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster02",
				Namespace: "default",
			},
			Spec: vlanv1alpha1.VLANDatabaseSpec{
				Kind: vlanv1alpha1.VLANDBKindESG,
			},
		},
		{
			TypeMeta: metav1.TypeMeta{
				APIVersion: vlanv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
				Kind:       vlanv1alpha1.VLANDatabaseKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster03",
				Namespace: "default",
			},
			Spec: vlanv1alpha1.VLANDatabaseSpec{
				Kind: vlanv1alpha1.VLANDBKindESG,
			},
		},
	},
}

func TestNetworkRun(t *testing.T) {
	cases := map[string]struct {
		CR              *infrav1alpha1.Network
		Config          *infrav1alpha1.NetworkConfig
		Endpoints       *endpoints.Endpoints
		Nodes           *nodes.Nodes
		vlanDBs         *vlanv1alpha1.VLANDatabaseList
		ExpectedDevices int
	}{
		"defaultSingleNode": {
			Config:          &infrav1alpha1.NetworkConfig{},
			Nodes:           singleNode,
			Endpoints:       testEndpointsSingleNode,
			vlanDBs:         vlanDBs,
			CR:              testDefaultCR,
			ExpectedDevices: 1,
		},
		"InternetSingleNode": {
			Config:          &infrav1alpha1.NetworkConfig{},
			Nodes:           singleNode,
			Endpoints:       testEndpointsSingleNode,
			vlanDBs:         vlanDBs,
			CR:              testInternetCR,
			ExpectedDevices: 1,
		},
		"defaultMultiNode": {
			Config:          &infrav1alpha1.NetworkConfig{},
			Nodes:           multiNode,
			Endpoints:       testEndpointsMultiNode,
			vlanDBs:         vlanDBs,
			CR:              testDefaultCR,
			ExpectedDevices: 3,
		},
		"InternetMultiNode": {
			Config:          &infrav1alpha1.NetworkConfig{},
			Nodes:           multiNode,
			Endpoints:       testEndpointsMultiNode,
			vlanDBs:         vlanDBs,
			CR:              testInternetCR,
			ExpectedDevices: 3,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resources := resources.New(
				resource.NewAPIPatchingApplicator(resource.NewMockClient()),
				resources.Config{
					CR:             tc.CR,
					MatchingLabels: allocv1alpha1.GetOwnerLabelsFromCR(tc.CR),
					Owns: []schema.GroupVersionKind{
						configv1alpha1.NetworkGroupVersionKind,
					},
				},
			)
			beipam, _ := ipambe.New(nil)
			ipamcp := ipam.NewBackendMock(beipam)
			bevlan, _ := vlanbe.New(nil)
			vlancp := vlan.NewBackendMock(bevlan)

			ctx := context.Background()

			for _, vlandb := range tc.vlanDBs.Items {
				vlancp.CreateIndex(ctx, &vlandb)
			}

			n := New(&Config{
				Config:    tc.Config,
				Apply:     true,
				Resources: resources,
				Endpoints: tc.Endpoints,
				Nodes:     tc.Nodes,
				Ipam:      ipamcl.NewIPAM(ipamcp),
				Vlan:      vlancl.NewVLAN(vlancp),
			})

			if err := n.Run(ctx, tc.CR); err != nil {
				assert.Error(t, err)
			}

			// Allocate resource
			for ref, o := range resources.GetNewResources() {
				if ref.Kind == vlanv1alpha1.VLANDatabaseKind {
					x, ok := o.(*vlanv1alpha1.VLANDatabase)
					if !ok {
						t.Errorf("expecting vlan Database, got: %v", reflect.TypeOf(o).Name())
					}
					if err := vlancp.CreateIndex(ctx, x); err != nil {
						assert.Error(t, err)
					}
				}
				if ref.Kind == ipamv1alpha1.NetworkInstanceKind {
					x, ok := o.(*ipamv1alpha1.NetworkInstance)
					if !ok {
						t.Errorf("expecting ipam NetworkInstance, got: %v", reflect.TypeOf(o).Name())
					}
					if err := ipamcp.CreateIndex(ctx, x); err != nil {
						assert.Error(t, err)
					}
					for _, prefix := range x.Spec.Prefixes {
						if _, err := ipamcp.Allocate(ctx, x, prefix); err != nil {
							assert.Error(t, err)
						}
					}
				}
			}
			n = New(&Config{
				Config:    tc.Config,
				Apply:     false,
				Resources: resources,
				Endpoints: tc.Endpoints,
				Nodes:     tc.Nodes,
				Ipam:      ipamcl.NewIPAM(ipamcp),
				Vlan:      vlancl.NewVLAN(vlancp),
			})
			if err := n.Run(ctx, tc.CR); err != nil {
				assert.NoError(t, err)
			}

			for _, rt := range tc.CR.Spec.RoutingTables {
				ni := ipamv1alpha1.BuildNetworkInstance(metav1.ObjectMeta{
					Name: rt.Name, Namespace: "default"},
					ipamv1alpha1.NetworkInstanceSpec{},
					ipamv1alpha1.NetworkInstanceStatus{})
				b, err := json.Marshal(ni)
				if err != nil {
					assert.NoError(t, err)
				}

				r, err := beipam.List(ctx, b)
				if err != nil {
					assert.NoError(t, err)
				}
				routes, ok := r.(table.Routes)
				if !ok {
					t.Errorf("expected table.Routes, got: %v", reflect.TypeOf(r))
				}
				for _, route := range routes {
					fmt.Println(route)
				}
			}

			if tc.ExpectedDevices != len(n.GetDevices()) {
				t.Errorf("expected %d, got %d", tc.ExpectedDevices, len(n.GetDevices()))
			}

			for nodeName, device := range n.GetDevices() {
				fmt.Println("nodeName: ", nodeName)

				j, err := ygot.EmitJSON(device, &ygot.EmitJSONConfig{
					Format: ygot.RFC7951,
					Indent: "  ",
					RFC7951Config: &ygot.RFC7951JSONConfig{
						AppendModuleName: true,
					},
					SkipValidation: false,
				})
				if err != nil {
					assert.NoError(t, err)
				}
				fmt.Println("config: ", string(j))
			}
		})
	}
}
