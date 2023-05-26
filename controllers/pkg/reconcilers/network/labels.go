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

// getSelectorLabels return the labels of the real data endpoint
// based on the leys we used to select the endpoints
func getSelectorLabels(labels map[string]string, keys []string) map[string]string {
	selectorLabels := map[string]string{}
	for k, v := range labels {
		for _, key := range keys {
			if k == key {
				selectorLabels[k] = v
			}
		}
	}
	return selectorLabels
}
