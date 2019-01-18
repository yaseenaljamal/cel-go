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
	"fmt"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter/functions"
)

// Dispatcher resolves function calls to their appropriate overload.
type Dispatcher interface {
	// Add one or more overloads, returning an error if any Overload has the
	// same Overload#Name.
	Add(overloads ...*functions.Overload) error

	// Dispatch a call to its appropriate Overload and set the evaluation
	// state for the call to the return value. The input state slice is
	// expected to contain the values indexed by expression id relevant to
	// the call args and the storage of the result.
	//
	// See: evalstate.go
	Dispatch(state []ref.Value, call *CallExpr)

	// FindOverload returns an Overload definition matching the provided
	// name.
	FindOverload(overload string) (*functions.Overload, bool)
}

// NewDispatcher returns an empty Dispatcher instance.
func NewDispatcher() Dispatcher {
	return &defaultDispatcher{
		overloads: make(map[string]*functions.Overload)}
}

// overloadMap helper type for indexing overloads by function name.
type overloadMap map[string]*functions.Overload

// defaultDispatcher struct which contains an overload map and a state
// instance used to track call args and return values.
type defaultDispatcher struct {
	overloads overloadMap
}

// Add implements the Dispatcher.Add interface method.
func (d *defaultDispatcher) Add(overloads ...*functions.Overload) error {
	for _, o := range overloads {
		// add the overload unless an overload of the same name has already
		// been provided before.
		if _, found := d.overloads[o.Operator]; found {
			return fmt.Errorf("overload already exists '%s'", o.Operator)
		}
		// Index the overload by function and by arg count.
		d.overloads[o.Operator] = o
	}
	return nil
}

// Dispatcher implements the Dispatcher.Dispatch interface method.
func (d *defaultDispatcher) Dispatch(state []ref.Value, call *CallExpr) {
	function := call.Function
	argCount := len(call.Args)
	overload, found := d.overloads[function]
	// Attempt to resolve the function as a receiver method.
	if !found {
		// Special dispatch for type-specific extension functions.
		if argCount == 0 {
			// If we're here, then there wasn't a zero-arg global function,
			// and there's definitely no member function without an operand.
			state[call.ID] = types.NewErr("no such overload")
			return
		}
		arg0 := state[call.Args[0]]
		if arg0.Type().HasTrait(traits.ReceiverType) {
			overload := call.Overload
			args := make([]ref.Value, argCount-1, argCount-1)
			for i, argID := range call.Args {
				if i == 0 {
					continue
				}
				args[i] = state[argID]
			}
			state[call.ID] = arg0.(traits.Receiver).Receive(function, overload, args)
			return
		}
		state[call.ID] = types.ValOrErr(arg0, "no such overload")
		return
	}
	// Attempt to invoke a global overload.
	if argCount == 0 {
		state[call.ID] = overload.Function()
		return
	}
	arg0 := state[call.Args[0]]
	if !arg0.Type().HasTrait(overload.OperandTrait) {
		state[call.ID] = types.ValOrErr(arg0, "no such overload")
		return
	}
	switch argCount {
	case 1:
		state[call.ID] = overload.Unary(arg0)
		return
	case 2:
		arg1 := state[call.Args[1]]
		state[call.ID] = overload.Binary(arg0, arg1)
		return
	case 3:
		arg1 := state[call.Args[1]]
		arg2 := state[call.Args[2]]
		state[call.ID] = overload.Function(arg0, arg1, arg2)
		return
	default:
		args := make([]ref.Value, argCount, argCount)
		for i, argID := range call.Args {
			args[i] = state[argID]
		}
		state[call.ID] = overload.Function(args...)
		return
	}
}

// FindOverload implements the Dispatcher.FindOverload interface method.
func (d *defaultDispatcher) FindOverload(overload string) (*functions.Overload, bool) {
	o, found := d.overloads[overload]
	return o, found
}
