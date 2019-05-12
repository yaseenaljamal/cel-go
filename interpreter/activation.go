// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interpreter

import (
	"errors"
	"fmt"
	"sync"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// Activation used to resolve identifiers by name and references by id.
//
// An Activation is the primary mechanism by which a caller supplies input into a CEL program.
type Activation interface {
	ExtendWith(bindings interface{}) (Activation, error)

	// Find returns a value from the activation by qualified name, or false if the name
	// could not be found.
	Find(name string) (interface{}, bool)

	// Parent returns the parent of the current activation, may be nil.
	// If non-nil, the parent will be searched during resolve calls.
	Parent() Activation

	Resolve(int64, CtxGetter) ref.Val
}

type CtxGetter interface {
   Get(Activation) interface{}
}

// EmptyActivation returns a variable free activation.
func EmptyActivation() Activation {
	// This call cannot fail.
	a, _ := NewActivation(map[string]interface{}{})
	return a
}

// NewActivation returns an activation based on a map-based binding where the map keys are
// expected to be qualified names used with ResolveName calls.
//
// The input `bindings` may either be of type `Activation` or `map[string]interface{}`.
//
// When the bindings are a `map` form whose values are not of `ref.Val` type, the values will be
// converted to CEL values (if possible) using the `types.DefaultTypeAdapter`.
func NewActivation(bindings interface{}) (Activation, error) {
	return NewAdaptingActivation(types.DefaultTypeAdapter, bindings)
}

// NewAdaptingActivation returns an actvation which is capable of adapting `bindings` from native
// Go values to equivalent CEL `ref.Val` objects.
//
// The input `bindings` may either be of type `Activation` or `map[string]interface{}`.
//
// When the bindings are a `map` the values may be one of the following types:
//   - `ref.Val`: a CEL value instance.
//   - `func() ref.Val`: a CEL value supplier.
//   - other: a native value which must be converted to a CEL `ref.Val` by the `adapter`.
func NewAdaptingActivation(adapter ref.TypeAdapter, bindings interface{}) (Activation, error) {
	if adapter == nil {
		return nil, errors.New("adapter must be non-nil")
	}
	if bindings == nil {
		return nil, errors.New("bindings must be non-nil")
	}
	a, isActivation := bindings.(Activation)
	if isActivation {
		return a, nil
	}
	m, isMap := bindings.(map[string]interface{})
	if !isMap {
		return nil, fmt.Errorf(
			"activation input must be an activation or map[string]interface: got %T",
			bindings)
	}
	return &mapActivation{adapter: adapter, bindings: m}, nil
}

// mapActivation which implements Activation and maps of named values.
//
// Named bindings may lazily supply values by providing a function which accepts no arguments and
// produces an interface value.
type mapActivation struct {
	adapter  ref.TypeAdapter
	bindings map[string]interface{}
	parent   Activation
}

func (a *mapActivation) ExtendWith(bindings interface{}) (Activation, error) {
	child, err := NewAdaptingActivation(a.adapter, bindings)
	if err != nil {
		return nil, err
	}
	curr := child.(*mapActivation)
	curr.parent = a
	return curr, nil
}

// Find implements the Activation interface method.
func (a *mapActivation) Find(name string) (interface{}, bool) {
	if object, found := a.bindings[name]; found {
		switch object.(type) {
		// Resolve a lazily bound value.
		case func() ref.Val:
			val := object.(func() ref.Val)()
			return val, true
		// Otherwise, return the bound value.
		default:
			return object, true
		}
	}
	if a.parent != nil {
		return a.parent.Find(name)
	}
	return nil, false
}

// Parent implements the Activation interface method.
func (a *mapActivation) Parent() Activation {
	return a.parent
}

func (a *mapActivation) Resolve(id int64, getter CtxGetter) ref.Val {
	return a.adapter.NativeToValue(getter.Get(a))
}

// varActivation represents a single mutable variable binding.
//
// This activation type should only be used within folds as the fold loop controls the object
// life-cycle.
type varActivation struct {
	parent Activation
	name   string
	val    ref.Val
}

// newVarActivation returns a new varActivation instance.
func newVarActivation(parent Activation, name string) *varActivation {
	return &varActivation{
		parent: parent,
		name:   name,
	}
}

// ExtendWith implements the Activation interface method.
func (v *varActivation) ExtendWith(bindings interface{}) (Activation, error) {
	panic("unexpected extension of varActivation")
}

// Find implements the Activation interface method.
func (v *varActivation) Find(name string) (interface{}, bool) {
	if name == v.name {
		return v.val, true
	}
	return v.parent.Find(name)
}

// Parent implements the Activation interface method.
func (v *varActivation) Parent() Activation {
	return v.parent
}

func (v *varActivation) Resolve(id int64, getter CtxGetter) ref.Val {
	obj := getter.Get(v)
	if obj != nil {
		return obj.(ref.Val)
	}
	return v.parent.Resolve(id, getter)
}

var (
	// pool of var activations to reduce allocations during folds.
	varActivationPool = &sync.Pool{
		New: func() interface{} {
			return &varActivation{}
		},
	}
)
