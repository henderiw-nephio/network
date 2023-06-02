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

package rootpaths

import (
	"fmt"
	"sort"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
)

// resolveRootSchemafromIntermediate retrieves the root schema from an lower level schema entry
// via iterating the parent entry up to the root
func resolveRootSchemafromIntermediate(se *yang.Entry) *yang.Entry {
	tmp := se
	for tmp.Parent != nil {
		tmp = tmp.Parent
	}
	return tmp
}

// CreateRootConfigElement - retrieves a Root or Device level configElement
func CreateRootConfigElement(yang_schema *yang.Entry) *configElement {
	ce := NewConfigElement("Device", nil, map[string]string{})
	ce.PathAndSchema = &PathAndSchema{
		path:   &gnmi.Path{},
		schema: yang_schema,
	}
	return ce
}

// getSortedKeyMapAsString takes a map[string]string struct of keys and respective values
// and generates a string representation of it. Maps are generally not sorted in golang
// but here we sort the output such that the order is stable and can even be used in hashCode()
func getSortedKeyMapAsString(keys_map map[string]string) string {
	result := ""

	if len(keys_map) > 0 {
		result += "["
		// maps are note sorted in golang, so we extrat the keys a alice of the keys that we can sort
		keys := make([]string, 0, len(keys_map))

		// add the keys
		for k := range keys_map {
			keys = append(keys, k)
		}
		// sort the keys array
		sort.Strings(keys)

		sep := ""
		// iterate over the map using the sorted keys
		for _, k := range keys {
			result += fmt.Sprintf("%s: %s%s", k, keys_map[k], sep)
			sep = ", "
		}
		result += "]"
	}
	return result
}

func ConfigElementHierarchyFromGnmiUpdate(yang_schema *yang.Entry, gn *gnmi.Notification) *configElement {
	rootConfigElement := CreateRootConfigElement(yang_schema)
	for _, path := range gn.GetDelete() {
		rootConfigElement.Add(GetPathAndSchemaEntry(yang_schema, path), nil)
	}
	for _, update := range gn.GetUpdate() {
		rootConfigElement.Add(GetPathAndSchemaEntry(yang_schema, update.Path), update.Val)
	}
	return rootConfigElement
}

// bool2string return a string representation of a bool ("true"/"false")
func bool2string(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
