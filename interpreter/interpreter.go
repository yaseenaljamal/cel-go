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

// Package interpreter provides functions to evaluate parsed expressions with
// the option to augment the evaluation with inputs and functions supplied at
// evaluation time.
package interpreter

import (
	"sync"

	"github.com/google/cel-go/common/packages"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter/functions"
)

// Interpreter generates a new Interpretable from a Program.
type Interpreter interface {
	// NewInterpretable returns an Interpretable from a Program.
	NewInterpretable(program *Program) Interpretable
}

// Interpretable can accept a given Activation and produce a value along with
// an accompanying EvalState which can be used to inspect whether additional
// data might be necessary to complete the evaluation.
type Interpretable interface {
	// Eval an Activation to produce an output and EvalState.
	Eval(activation Activation) (ref.Value, EvalState)
}

type exprInterpreter struct {
	dispatcher   Dispatcher
	packager     packages.Packager
	typeProvider ref.TypeProvider
}

// NewInterpreter builds an Interpreter from a Dispatcher and TypeProvider
// which will be used throughout the Eval of all Interpretable instances
// gerenated from it.
func NewInterpreter(dispatcher Dispatcher,
	packager packages.Packager,
	typeProvider ref.TypeProvider) Interpreter {
	return &exprInterpreter{
		dispatcher:   dispatcher,
		packager:     packager,
		typeProvider: typeProvider}
}

// NewStandardInterpreter builds a Dispatcher and TypeProvider with support
// for all of the CEL builtins defined in the language definition.
func NewStandardInterpreter(packager packages.Packager,
	typeProvider ref.TypeProvider) Interpreter {
	dispatcher := NewDispatcher()
	dispatcher.Add(functions.StandardOverloads()...)
	return NewInterpreter(dispatcher, packager, typeProvider)
}

func (i *exprInterpreter) NewInterpretable(program *Program) Interpretable {
	// program needs to be pruned with the TypeProvider
	staticState := newEvalState(program.MaxInstructionID() + 1)
	intr := &exprInterpreter{
		dispatcher:   i.dispatcher,
		packager:     i.packager,
		typeProvider: i.typeProvider}
	program.plan(staticState)
	program.bindOperators(i.dispatcher)
	program.computeConstExprs(staticState)
	statePool := &sync.Pool{
		New: func() interface{} {
			runtimeState := make([]ref.Value, staticState.exprCount, staticState.exprCount)
			copy(runtimeState, staticState.values)
			return runtimeState
		},
	}
	return &exprInterpretable{
		interpreter: intr,
		program:     program,
		staticState: staticState,
		statePool:   statePool,
	}
}

type exprInterpretable struct {
	interpreter *exprInterpreter
	program     *Program
	staticState *evalState
	statePool   *sync.Pool
}

// allocState will return an array suitable for tracking runtime state. The static state derived
// at program plan time will be copied into the runtime state.
//
// By default, state arrays are pooled within a thread-safe sync.Pool; however, when state
// tracking is enabled via programFlag, a new array is allocated. Pooled state objects may be
// dirty in the sense that they will contain fragments of state from previous executions against
// potentially different inputs; this is safe when the state does not need to be exposed, as the
// expression result will always overwrite the state values relevant to the current computation.
func (i *exprInterpretable) allocState() []ref.Value {
	if i.program.flags&programFlagTrackState == programFlagTrackState {
		rawState := make([]ref.Value, i.staticState.exprCount, i.staticState.exprCount)
		copy(rawState, i.staticState.values)
		return rawState
	}
	return i.statePool.Get().([]ref.Value)
}

// finalizeState will either release the current runtime state back to a sync.Pool, or copy it
// into a new hermetic evalState instance suitable for sharing with external callers. When state
// tracking is enabled the result will be non-nil; otherwise the result will be nil by default.
func (i *exprInterpretable) finalizeState(state []ref.Value) *evalState {
	if i.program.flags&programFlagTrackState == programFlagTrackState {
		finalState := newEvalState(i.staticState.exprCount)
		finalState.exprIDMap = i.staticState.exprIDMap
		finalState.values = state
		return finalState
	}
	i.statePool.Put(state)
	return nil
}

// Eval is an implementation of the Interpretable interface method.
func (i *exprInterpretable) Eval(activation Activation) (ref.Value, EvalState) {
	// register machine-like evaluation of the program with the given activation.
	currActivation := activation
	d := i.interpreter.dispatcher

	// program counter and execution state.
	pc := 0
	end := len(i.program.Instructions)
	steps := i.program.Instructions
	if pc == end {
		return i.staticState.values[i.program.resultID], i.staticState
	}
	state := i.allocState()

	for pc < end {
		step := steps[pc]
		switch step.(type) {
		case *IdentExpr:
			i.evalIdent(state, step.(*IdentExpr), currActivation)
		case *SelectExpr:
			i.evalSelect(state, step.(*SelectExpr), currActivation)
		case *CallExpr:
			d.Dispatch(state, step.(*CallExpr))
		case *CreateListExpr:
			i.evalCreateList(state, step.(*CreateListExpr))
		case *CreateMapExpr:
			i.evalCreateMap(state, step.(*CreateMapExpr))
		case *CreateObjectExpr:
			i.evalCreateType(state, step.(*CreateObjectExpr))
		case *MovInst:
			i.evalMov(state, step.(*MovInst))
			// Special instruction for modifying the program cursor
		case *JumpInst:
			jmpExpr := step.(*JumpInst)
			if jmpExpr.OnCondition(state) {
				jmpPc := pc + jmpExpr.Count
				if jmpPc < 0 || jmpPc > end {
					// TODO: Error, the jump count should never exceed the
					// program length.
					panic("jumped too far")
				}
				pc = jmpPc
				continue
			}
			// Special instructions for modifying the activation stack
		case *PushScopeInst:
			pushScope := step.(*PushScopeInst)
			scopeDecls := pushScope.Declarations
			childActivaton := make(map[string]interface{})
			for key, declID := range scopeDecls {
				childActivaton[key] = func() interface{} {
					return i.value(state, declID)
				}
			}
			currActivation =
				NewHierarchicalActivation(currActivation, NewActivation(childActivaton))
		case *PopScopeInst:
			currActivation = currActivation.Parent()
		}
		pc++
	}
	result := i.value(state, i.program.resultID)
	return result, i.finalizeState(state)
}

func (i *exprInterpretable) evalIdent(state []ref.Value,
	idExpr *IdentExpr,
	currActivation Activation) {
	// TODO: Refactor this code for sharing.
	if result, found := currActivation.ResolveName(idExpr.Name); found {
		state[idExpr.ID] = result
		return
	}
	if idVal, found := i.interpreter.typeProvider.FindIdent(idExpr.Name); found {
		state[idExpr.ID] = idVal
		return
	}
	state[idExpr.ID] = types.Unknown{idExpr.ID}
}

func (i *exprInterpretable) evalSelect(state []ref.Value,
	selExpr *SelectExpr,
	currActivation Activation) {
	operand := i.value(state, selExpr.Operand)
	if !operand.Type().HasTrait(traits.IndexerType) {
		// If the operand is unknown, this could be an identifer.
		if types.IsUnknown(operand) {
			resVal := i.resolveUnknown(
				state, operand.(types.Unknown), selExpr, currActivation)
			state[selExpr.ID] = resVal
			return
		}
		// If the operand is an error, early return.
		if types.IsError(operand) {
			state[selExpr.ID] = operand
			return
		}
		// Otherwise, create an error.
		state[selExpr.ID] = types.NewErr("invalid operand in select")
		return
	}
	field := types.String(selExpr.Field)
	if selExpr.TestOnly {
		if operand.Type() == types.MapType {
			state[selExpr.ID] = operand.(traits.Container).Contains(field)
			return
		}
		if operand.Type().HasTrait(traits.FieldTesterType) {
			state[selExpr.ID] = operand.(traits.FieldTester).IsSet(field)
			return
		}
		state[selExpr.ID] = types.NewErr("invalid operand in select")
		return
	}
	fieldValue := operand.(traits.Indexer).Get(field)
	state[selExpr.ID] = fieldValue
}

// resolveUnknown attempts to resolve a qualified name from a select expression
// which may have generated unknown values during the course of execution if
// the expression was not type-checked and the select, in fact, refers to a
// qualified identifier name instead of a series of field selections.
//
// Returns one of the following:
// - The resolved identifier value from the activation
// - An unknown value if the expression is a valid identifier, but was not found.
// - Otherwise, an error.
func (i *exprInterpretable) resolveUnknown(state []ref.Value,
	unknown types.Unknown,
	selExpr *SelectExpr,
	currActivation Activation) ref.Value {
	if object, found := currActivation.ResolveReference(selExpr.ID); found {
		return object
	}
	validIdent := true
	identifier := selExpr.Field
	for _, arg := range unknown {
		inst := i.program.GetInstruction(arg)
		switch inst.(type) {
		case *IdentExpr:
			identifier = inst.(*IdentExpr).Name + "." + identifier
		case *SelectExpr:
			identifier = inst.(*SelectExpr).Field + "." + identifier
		default:
			argVal := i.value(state, arg)
			if argVal.Type() == types.StringType {
				identifier = string(argVal.(types.String)) + "." + identifier
			} else {
				validIdent = false
				break
			}
		}
	}
	if !validIdent {
		return types.NewErr("invalid identifier encountered: %v", selExpr)
	}
	pkg := i.interpreter.packager
	tp := i.interpreter.typeProvider
	for _, id := range pkg.ResolveCandidateNames(identifier) {
		if object, found := currActivation.ResolveName(id); found {
			return object
		}
		if identVal, found := tp.FindIdent(id); found {
			return identVal
		}
	}
	return append(types.Unknown{selExpr.ID}, unknown...)
}

func (i *exprInterpretable) evalCreateList(state []ref.Value,
	listExpr *CreateListExpr) {
	elements := make([]ref.Value, len(listExpr.Elements))
	for idx, elementID := range listExpr.Elements {
		elem := i.value(state, elementID)
		if types.IsUnknownOrError(elem) {
			state[listExpr.ID] = elem
			return
		}
		elements[idx] = i.value(state, elementID)
	}
	adaptingList := types.NewDynamicList(elements)
	state[listExpr.ID] = adaptingList
}

func (i *exprInterpretable) evalCreateMap(state []ref.Value,
	mapExpr *CreateMapExpr) {
	entries := make(map[ref.Value]ref.Value)
	for keyID, valueID := range mapExpr.KeyValues {
		key := i.value(state, keyID)
		if types.IsUnknownOrError(key) {
			state[mapExpr.ID] = key
			return
		}
		val := i.value(state, valueID)
		if types.IsUnknownOrError(val) {
			state[mapExpr.ID] = val
			return
		}
		entries[key] = val
	}
	adaptingMap := types.NewDynamicMap(entries)
	state[mapExpr.ID] = adaptingMap
}

func (i *exprInterpretable) evalCreateType(state []ref.Value,
	objExpr *CreateObjectExpr) {
	fields := make(map[string]ref.Value)
	for field, valueID := range objExpr.FieldValues {
		val := i.value(state, valueID)
		if types.IsUnknownOrError(val) {
			state[objExpr.ID] = val
			return
		}
		fields[field] = val
	}
	state[objExpr.ID] = i.newValue(objExpr.Name, fields)
}

func (i *exprInterpretable) evalMov(state []ref.Value, movExpr *MovInst) {
	state[movExpr.ToExprID] = i.value(state, movExpr.ID)
}

func (i *exprInterpretable) value(state []ref.Value, id int64) ref.Value {
	if object := state[id]; object != nil {
		return object
	}
	return types.Unknown{id}
}

func (i *exprInterpretable) newValue(typeName string,
	fields map[string]ref.Value) ref.Value {
	pkg := i.interpreter.packager
	tp := i.interpreter.typeProvider
	for _, qualifiedTypeName := range pkg.ResolveCandidateNames(typeName) {
		if _, found := tp.FindType(qualifiedTypeName); found {
			typeName = qualifiedTypeName
			break
		}
	}
	return i.interpreter.typeProvider.NewValue(typeName, fields)
}
