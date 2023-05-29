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
	"sort"

	"github.com/openconfig/gnmi/proto/gnmi"
)

func GetDeletePaths(gnmiPaths []*gnmi.Path) []*gnmi.Path {
	// check if the pathElem Name is a key
	for _, p := range gnmiPaths {
		if len(p.Elem) > 1 {
			// check if the last pathElem Name is a key
			if len(p.Elem[len(p.Elem)-2].Key) != 0 {
				if _, ok := p.Elem[len(p.Elem)-2].Key[p.Elem[len(p.Elem)-1].Name]; ok {
					p.Elem = p.Elem[:(len(p.Elem) - 1)]
				}
			}
		}
	}

	// sort the slice to ensure we can ahndle the overlap properly
	sort.Slice(gnmiPaths, func(i, j int) bool {
		return len(gnmiPaths[i].Elem) < len(gnmiPaths[j].Elem)
	})

	// check if the pathElem are contained
	deletePaths := []*gnmi.Path{}
	for _, p := range gnmiPaths {
		overlaps := false // ensures the first path is added as a deletepath since nothing exists
		for _, dp := range deletePaths {
			overlaps = true
			for i := 0; i < len(dp.Elem); i++ {
				if dp.Elem[i].Name != p.Elem[i].Name {
					overlaps = false
					break
				}
				for k, v := range dp.Elem[i].Key {
					val, ok := p.Elem[i].Key[k]
					if !ok {
						overlaps = false
						break
					}
					if val != v {
						overlaps = false
						break
					}
				}
				if !overlaps {
					break
				}
			}
			if overlaps {
				break
			}
		}
		if !overlaps {
			// no overlap found -> this path should be deleted
			deletePaths = append(deletePaths, p)
		}
	}
	return deletePaths
}
