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

	"github.com/openconfig/gnmi/proto/gnmi"
)

// configElement is used to carry all the config and its related schema data while
// representing it in a tree hierarchy
type configElement struct {
	// reference to the path and schema
	*PathAndSchema
	// all the child configElements
	children map[string]*configElement
	// reference to the higher level / parent configElement
	parent *configElement
	// name of the actual configElement
	name string
	// keys and key values of the configElements PathElement
	keys map[string]string
	// the value of leaf elements
	value *gnmi.TypedValue
}

// NewConfigElement helper function for the generation of new configElements
func NewConfigElement(name string, parent *configElement, keys map[string]string) *configElement {
	return &configElement{name: name, children: map[string]*configElement{}, parent: parent, PathAndSchema: nil, keys: keys, value: nil}
}

// getHierarchicalOutput_internal produces a hierarchical string representation from this along
// all child configElements down to the leafs.
func (ce *configElement) GetHierarchicalOutput(indentor string) string {
	return ce.getHierarchicalOutput_internal(0, indentor)
}

// getHierarchicalOutput_internal produces a hierarchical string representation from this along
// all child configElements down to the leafs.
func (ce *configElement) getHierarchicalOutput_internal(level int, indentor string) string {
	result := ""
	indent := ""
	for i := 0; i <= level; i++ {
		indent += indentor
	}
	result += fmt.Sprintf("%s %s %s = %s {hasNonKeyChilds: %s, IsLeaf: %s, ChildCount: %d, IsKey: %s, hasOnlyKeyChilds: %s, hasLeafChilds: %s, hasNonKeyLeafChilds: %s, isDefault: %s}\n",
		indent,
		ce.name,
		ce.getKeysAsString(),
		ce.value.GetStringVal(),
		bool2string(ce.hasNonKeyChilds()),
		bool2string(ce.isLeaf()),
		ce.getChildCount(),
		bool2string(ce.isKey()),
		bool2string(ce.hasOnlyKeyChilds()),
		bool2string(ce.hasLeafChilds()),
		bool2string(ce.hasNonKeyAndDefaultLeafChilds()),
		bool2string(ce.isDefault()),
	)
	for _, cce := range ce.children {
		result += cce.getHierarchicalOutput_internal(level+1, indentor)
	}
	return result
}

// isKey checks if the pathElement is used as a key in the parent configElement
func (ce *configElement) isKey() bool {
	result := false
	if ce.parent == nil {
		// on root node return false
		return false
	}
	// check the keys of the parent configElement if the names is listed there
	for k := range ce.parent.keys {
		if ce.name == k {
			result = true
		}
	}
	return result
}

// isLeaf checks if this configElement is the Terminal Leaf or if multiple
// sub values exist
func (ce *configElement) isLeaf() bool {
	return len(ce.children) <= 0
}

func (ce *configElement) isDefault() bool {
	for _, defval := range ce.schema.Default {
		if ce.value.GetStringVal() == defval {
			return true
		}
	}
	return false
}

// GetRootPaths determines the segnificant, the RootPaths out of the
// configElements structure
func (ce *configElement) GetRootPaths() []*gnmi.Path {
	result := []*gnmi.Path{}

	// skip leaf elements, they should not appear in the result
	if ce.isLeaf() {
		return result
	}
	if ce.hasNonKeyAndDefaultLeafChilds() {
		// if the config Element has Childs which are not used as Key, add it's path
		result = append(result, ce.path)
	} else if ce.hasOnlyKeyChilds() {
		// if there is only a key configElement listed without any non-key leafs,
		// it must also be significant
		result = append(result, ce.path)
	} else {
		// if non of the conditions atop where true, continue the same (recursive checks with the child configElements)
		for _, v := range ce.children {
			result = append(result, v.GetRootPaths()...)
		}
	}
	return result
}

// hasNonKeyAndDefaultLeafChilds checks if there is any Leaf child that is not used as a key and does not carry the default value
func (ce *configElement) hasNonKeyAndDefaultLeafChilds() bool {
	for _, c := range ce.children {
		if c.isLeaf() && !c.isKey() && !c.isDefault() {
			return true
		}
	}
	return false
}

// hasOnlyKeyChilds checks if the configElement holds only configElements which are used as keys.
// no other non-key value is defined
func (ce *configElement) hasOnlyKeyChilds() bool {
	for _, c := range ce.children {
		if !c.isKey() {
			return false
		}
	}
	return true && len(ce.children) > 0
}

// getKeysAsString returns the key map in its string representation.
// The key order is stable since the Keys are being sorted before iteration.
func (ce *configElement) getKeysAsString() string {
	return getSortedKeyMapAsString(ce.keys)
}

func (ce *configElement) getHierarchicalName() string {
	parentname := ""
	if ce.parent != nil {
		parentname = ce.parent.getHierarchicalName() + "->"
	}

	return parentname + ce.name + ce.getKeysAsString()
}

func (ce *configElement) String() string {
	return fmt.Sprintf("%s Level: %d", ce.getHierarchicalName(), ce.getLevel())
}

// getLevel returns the level from the SchemaRoot.
// the first level under the device, like {interface, system, or the like} is treated as level 0
func (ce *configElement) getLevel() int {
	if ce.parent == nil {
		return -1
	}
	parentlevel := ce.parent.getLevel()
	return parentlevel + 1
}

// Add to be called on the Root configElem with PathAndSchema elements to Add them to the hierarchical tree.
func (ce *configElement) Add(pas *PathAndSchema, value *gnmi.TypedValue) {
	// delegate to add_internal enriching call with index = 0
	ce.add_internal(pas, value, 0)
}

// Add to be called on the Root configElem with PathAndSchema elements to Add them to the hierarchical tree.
func (ce *configElement) add_internal(pas *PathAndSchema, value *gnmi.TypedValue, index int) {
	// stop recursion if we are beyond the last PathElem
	if len(pas.path.Elem)-1 == ce.getLevel() {
		return
	}
	// get the name of the actual level, meaning PathElem
	name := pas.path.Elem[index].Name
	// wrap the PathElem in MyPathElem struct
	elemPath := &MyPathElem{pas.path.Elem[index]}

	// check if a child with the generated hashCode() already exists
	newCE, exists := ce.children[elemPath.hashCode()]

	// if it does not, create it and add it as a child element
	if !exists {
		newCE = NewConfigElement(name, ce, elemPath.Key) // generate a new configElement with all required Information
		// set the Path and Schema information appropriately
		newCE.PathAndSchema = GetPathAndSchemaEntry(resolveRootSchemafromIntermediate(pas.schema), &gnmi.Path{Elem: pas.path.Elem[0 : index+1]})
		// add the created configElement as a child
		ce.children[elemPath.hashCode()] = newCE
		if len(pas.path.Elem)-1 == newCE.getLevel() {
			newCE.value = value
		}

	}
	// decent down the path in recursion, increasinbg the level via index
	newCE.add_internal(pas, value, index+1)
}

// hasLeafChilds checks is the configElement has terminal / leaf childs
func (ce *configElement) hasLeafChilds() bool {
	for _, c := range ce.children {
		if c.isLeaf() {
			return true
		}
	}
	return false
}

// getChildCount returns the number of child configElements
func (ce *configElement) getChildCount() int {
	return len(ce.children)
}

// hasNonKeyChilds checks if the configElement has child elements which are not
// used as keys to identify the Element
func (ce *configElement) hasNonKeyChilds() bool {
	result := false
	if ce.isLeaf() {
		return !ce.isKey()
	} else {
		for _, v := range ce.children {
			result = result || v.hasNonKeyChilds()
		}
	}

	return result
}
