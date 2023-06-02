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

import "github.com/openconfig/gnmi/proto/gnmi"

// MyPathElem - A wrapper for the gnmi.PathElem.
// Used to be able to implement a custom hashCode() method, to be able to allow for map lookups
// not just on identity (pointer equality) but in this case, name and key equality
type MyPathElem struct {
	*gnmi.PathElem
}

// hashCode defines the hashCode calculation for the MyPathElem, which is a wrapper around the gnmi.PathElem.
// The hashCode is the name concatinated with the keys as key values.
func (pe *MyPathElem) hashCode() string {
	// returning the generated hashCode
	return pe.Name + getSortedKeyMapAsString(pe.Key)
}
