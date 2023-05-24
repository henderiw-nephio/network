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

package v1alpha1

import (
	"reflect"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NetworkSpec defines the desired state of Network
type NetworkSpec struct {
	Topology      string         `json:"topology" yaml:"topology"`
	BridgeDomains []BridgeDomain `json:"bridgeDomains,omitempty" yaml:"bridgeDomains,omitempty"`
	RoutingTables []RoutingTable `json:"routingTables,omitempty" yaml:"routingTables,omitempty"`
}

type BridgeDomain struct {
	Name       string      `json:"name" yaml:"name"`
	Interfaces []Interface `json:"interfaces,omitempty" yaml:"interfaces,omitempty"`
}

type RoutingTable struct {
	Name          string                `json:"name" yaml:"name"`
	Interfaces    []Interface           `json:"interfaces,omitempty" yaml:"interfaces,omitempty"`
	BridgeDomains []Interface           `json:"bridgeDomains,omitempty" yaml:"bridgeDomains,omitempty"`
	Prefixes      []ipamv1alpha1.Prefix `json:"prefixes" yaml:"prefixes"`
}

type Interface struct {
	// Name defines the name of the interface or name of the bridge domain. When using a bridgedoamin
	// this parameter is mandatory
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// Selector defines the selector criterias for the interface selection
	Selector metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
}

// NetworkStatus defines the observed state of Network
type NetworkStatus struct {
	// ConditionedStatus provides the status of the Readiness using conditions
	// if the condition is true the other attributes in the status are meaningful
	ConditionedStatus `json:",inline" yaml:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="REPO_STATUS",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"

// Network is the Schema for the Network API
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec,omitempty"`
	Status NetworkStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NetworkList contains a list of Repositories
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}

// Network type metadata.
var (
	NetworkKind             = reflect.TypeOf(Network{}).Name()
	NetworkGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: NetworkKind}.String()
	NetworkKindAPIVersion   = NetworkKind + "." + GroupVersion.String()
	NetworkGroupVersionKind = GroupVersion.WithKind(NetworkKind)
)
