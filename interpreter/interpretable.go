// Copyright 2019 Google LLC
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

// Interpretable can accept a given Activation and produce a value along with
// an accompanying EvalState which can be used to inspect whether additional
// data might be necessary to complete the evaluation.
type Interpretable interface {
	// ID value corresponding to the expression node.
	ID() int64

	// Eval an Activation to produce an output.
	Eval(vars Activation) ref.Val
}

// Core Interpretable implementations used during the program planning phase.

type evalTestOnly struct {
	id    int64
	op    Interpretable
	field types.String
}

// ID implements the Interpretable interface method.
func (test *evalTestOnly) ID() int64 {
	return test.id
}

// Eval implements the Interpretable interface method.
func (test *evalTestOnly) Eval(vars Activation) ref.Val {
	obj := test.op.Eval(vars)
	tester, ok := obj.(traits.FieldTester)
	if ok {
		return tester.IsSet(test.field)
	}
	container, ok := obj.(traits.Container)
	if ok {
		return container.Contains(test.field)
	}
	return types.ValOrErr(obj, "invalid type for field selection.")

}

type evalConst struct {
	id  int64
	val ref.Val
}

// ID implements the Interpretable interface method.
func (cons *evalConst) ID() int64 {
	return cons.id
}

// Eval implements the Interpretable interface method.
func (cons *evalConst) Eval(vars Activation) ref.Val {
	return cons.val
}

type evalOr struct {
	id  int64
	lhs Interpretable
	rhs Interpretable
}

// ID implements the Interpretable interface method.
func (or *evalOr) ID() int64 {
	return or.id
}

// Eval implements the Interpretable interface method.
func (or *evalOr) Eval(vars Activation) ref.Val {
	// short-circuit lhs.
	lVal := or.lhs.Eval(vars)
	lBool, lok := lVal.(types.Bool)
	if lok && lBool == types.True {
		return types.True
	}
	// short-circuit on rhs.
	rVal := or.rhs.Eval(vars)
	rBool, rok := rVal.(types.Bool)
	if rok && rBool == types.True {
		return types.True
	}
	// return if both sides are bool false.
	if lok && rok {
		return types.False
	}
	// TODO: return both values as a set if both are unknown or error.
	// prefer left unknown to right unknown.
	if types.IsUnknown(lVal) {
		return lVal
	}
	if types.IsUnknown(rVal) {
		return rVal
	}
	// if the left-hand side is non-boolean return it as the error.
	return types.ValOrErr(lVal, "no such overload")
}

type evalAnd struct {
	id  int64
	lhs Interpretable
	rhs Interpretable
}

// ID implements the Interpretable interface method.
func (and *evalAnd) ID() int64 {
	return and.id
}

// Eval implements the Interpretable interface method.
func (and *evalAnd) Eval(vars Activation) ref.Val {
	// short-circuit lhs.
	lVal := and.lhs.Eval(vars)
	lBool, lok := lVal.(types.Bool)
	if lok && lBool == types.False {
		return types.False
	}
	// short-circuit on rhs.
	rVal := and.rhs.Eval(vars)
	rBool, rok := rVal.(types.Bool)
	if rok && rBool == types.False {
		return types.False
	}
	// return if both sides are bool true.
	if lok && rok {
		return types.True
	}
	// TODO: return both values as a set if both are unknown or error.
	// prefer left unknown to right unknown.
	if types.IsUnknown(lVal) {
		return lVal
	}
	if types.IsUnknown(rVal) {
		return rVal
	}
	// if the left-hand side is non-boolean return it as the error.
	return types.ValOrErr(lVal, "no such overload")
}

type evalConditional struct {
	id     int64
	expr   Interpretable
	truthy Interpretable
	falsy  Interpretable
}

// ID implements the Interpretable interface method.
func (cond *evalConditional) ID() int64 {
	return cond.id
}

// Eval implements the Interpretable interface method.
func (cond *evalConditional) Eval(vars Activation) ref.Val {
	condVal := cond.expr.Eval(vars)
	condBool, ok := condVal.(types.Bool)
	if !ok {
		return types.ValOrErr(condVal, "no such overload")
	}
	if condBool {
		return cond.truthy.Eval(vars)
	}
	return cond.falsy.Eval(vars)
}

type evalEq struct {
	id  int64
	lhs Interpretable
	rhs Interpretable
}

// ID implements the Interpretable interface method.
func (eq *evalEq) ID() int64 {
	return eq.id
}

// Eval implements the Interpretable interface method.
func (eq *evalEq) Eval(vars Activation) ref.Val {
	lVal := eq.lhs.Eval(vars)
	rVal := eq.rhs.Eval(vars)
	return lVal.Equal(rVal)
}

type evalNe struct {
	id  int64
	lhs Interpretable
	rhs Interpretable
}

// ID implements the Interpretable interface method.
func (ne *evalNe) ID() int64 {
	return ne.id
}

// Eval implements the Interpretable interface method.
func (ne *evalNe) Eval(vars Activation) ref.Val {
	lVal := ne.lhs.Eval(vars)
	rVal := ne.rhs.Eval(vars)
	eqVal := lVal.Equal(rVal)
	eqBool, ok := eqVal.(types.Bool)
	if !ok {
		if types.IsUnknown(eqVal) {
			return eqVal
		}
		return types.NewErr("no such overload")
	}
	return !eqBool
}

type evalZeroArity struct {
	id   int64
	impl functions.FunctionOp
}

// ID implements the Interpretable interface method.
func (zero *evalZeroArity) ID() int64 {
	return zero.id
}

// Eval implements the Interpretable interface method.
func (zero *evalZeroArity) Eval(vars Activation) ref.Val {
	return zero.impl()
}

type evalUnary struct {
	id       int64
	function string
	overload string
	arg      Interpretable
	trait    int
	impl     functions.UnaryOp
}

// ID implements the Interpretable interface method.
func (un *evalUnary) ID() int64 {
	return un.id
}

// Eval implements the Interpretable interface method.
func (un *evalUnary) Eval(vars Activation) ref.Val {
	argVal := un.arg.Eval(vars)
	// Early return if the argument to the function is unknown or error.
	if types.IsUnknownOrError(argVal) {
		return argVal
	}
	// If the implementation is bound and the argument value has the right traits required to
	// invoke it, then call the implementation.
	if un.impl != nil && (un.trait == 0 || argVal.Type().HasTrait(un.trait)) {
		return un.impl(argVal)
	}
	// Otherwise, if the argument is a ReceiverType attempt to invoke the receiver method on the
	// operand (arg0).
	if argVal.Type().HasTrait(traits.ReceiverType) {
		return argVal.(traits.Receiver).ReceiveUnary(un.function, un.overload)
	}
	return types.NewErr("no such overload: %s", un.function)
}

type evalBinary struct {
	id       int64
	function string
	overload string
	lhs      Interpretable
	rhs      Interpretable
	trait    int
	impl     functions.BinaryOp
}

// ID implements the Interpretable interface method.
func (bin *evalBinary) ID() int64 {
	return bin.id
}

// Eval implements the Interpretable interface method.
func (bin *evalBinary) Eval(vars Activation) ref.Val {
	lVal := bin.lhs.Eval(vars)
	rVal := bin.rhs.Eval(vars)
	// Early return if any argument to the function is unknown or error.
	if types.IsUnknownOrError(lVal) {
		return lVal
	}
	if types.IsUnknownOrError(rVal) {
		return rVal
	}
	// If the implementation is bound and the argument value has the right traits required to
	// invoke it, then call the implementation.
	if bin.impl != nil && (bin.trait == 0 || lVal.Type().HasTrait(bin.trait)) {
		return bin.impl(lVal, rVal)
	}
	// Otherwise, if the argument is a ReceiverType attempt to invoke the receiver method on the
	// operand (arg0).
	if lVal.Type().HasTrait(traits.ReceiverType) {
		return lVal.(traits.Receiver).ReceiveBinary(bin.function, bin.overload, rVal)
	}
	return types.NewErr("no such overload: %s", bin.function)
}

type evalVarArgs struct {
	id       int64
	function string
	overload string
	args     []Interpretable
	trait    int
	impl     functions.FunctionOp
}

// ID implements the Interpretable interface method.
func (fn *evalVarArgs) ID() int64 {
	return fn.id
}

// Eval implements the Interpretable interface method.
func (fn *evalVarArgs) Eval(vars Activation) ref.Val {
	argVals := make([]ref.Val, len(fn.args), len(fn.args))
	// Early return if any argument to the function is unknown or error.
	for i, arg := range fn.args {
		argVals[i] = arg.Eval(vars)
		if types.IsUnknownOrError(argVals[i]) {
			return argVals[i]
		}
	}
	// If the implementation is bound and the argument value has the right traits required to
	// invoke it, then call the implementation.
	arg0 := argVals[0]
	if fn.impl != nil && (fn.trait == 0 || arg0.Type().HasTrait(fn.trait)) {
		return fn.impl(argVals...)
	}
	// Otherwise, if the argument is a ReceiverType attempt to invoke the receiver method on the
	// operand (arg0).
	if arg0.Type().HasTrait(traits.ReceiverType) {
		return arg0.(traits.Receiver).Receive(fn.function, fn.overload, argVals[1:])
	}
	return types.NewErr("no such overload: %s", fn.function)
}

type evalList struct {
	id      int64
	elems   []Interpretable
	adapter ref.TypeAdapter
}

// ID implements the Interpretable interface method.
func (l *evalList) ID() int64 {
	return l.id
}

// Eval implements the Interpretable interface method.
func (l *evalList) Eval(vars Activation) ref.Val {
	elemVals := make([]ref.Val, len(l.elems), len(l.elems))
	// If any argument is unknown or error early terminate.
	for i, elem := range l.elems {
		elemVal := elem.Eval(vars)
		if types.IsUnknownOrError(elemVal) {
			return elemVal
		}
		elemVals[i] = elemVal
	}
	return types.NewDynamicList(l.adapter, elemVals)
}

type evalMap struct {
	id      int64
	keys    []Interpretable
	vals    []Interpretable
	adapter ref.TypeAdapter
}

// ID implements the Interpretable interface method.
func (m *evalMap) ID() int64 {
	return m.id
}

// Eval implements the Interpretable interface method.
func (m *evalMap) Eval(vars Activation) ref.Val {
	entries := make(map[ref.Val]ref.Val)
	// If any argument is unknown or error early terminate.
	for i, key := range m.keys {
		keyVal := key.Eval(vars)
		if types.IsUnknownOrError(keyVal) {
			return keyVal
		}
		valVal := m.vals[i].Eval(vars)
		if types.IsUnknownOrError(valVal) {
			return valVal
		}
		entries[keyVal] = valVal
	}
	return types.NewDynamicMap(m.adapter, entries)
}

type evalObj struct {
	id       int64
	typeName string
	fields   []string
	vals     []Interpretable
	provider ref.TypeProvider
}

// ID implements the Interpretable interface method.
func (o *evalObj) ID() int64 {
	return o.id
}

// Eval implements the Interpretable interface method.
func (o *evalObj) Eval(vars Activation) ref.Val {
	fieldVals := make(map[string]ref.Val)
	// If any argument is unknown or error early terminate.
	for i, field := range o.fields {
		val := o.vals[i].Eval(vars)
		if types.IsUnknownOrError(val) {
			return val
		}
		fieldVals[field] = val
	}
	return o.provider.NewValue(o.typeName, fieldVals)
}

type evalFold struct {
	id        int64
	accuVar   string
	iterVar   string
	iterRange Interpretable
	accu      Interpretable
	cond      Interpretable
	step      Interpretable
	result    Interpretable
}

// ID implements the Interpretable interface method.
func (fold *evalFold) ID() int64 {
	return fold.id
}

// Eval implements the Interpretable interface method.
func (fold *evalFold) Eval(vars Activation) ref.Val {
	foldRange := fold.iterRange.Eval(vars)
	if !foldRange.Type().HasTrait(traits.IterableType) {
		return types.ValOrErr(foldRange, "got '%T', expected iterable type", foldRange)
	}
	// Configure the fold activation with the accumulator initial value.
	accuCtx := varActivationPool.Get().(*varActivation)
	accuCtx.parent = vars
	accuCtx.name = fold.accuVar
	accuCtx.val = fold.accu.Eval(vars)
	iterCtx := varActivationPool.Get().(*varActivation)
	iterCtx.parent = accuCtx
	iterCtx.name = fold.iterVar
	it := foldRange.(traits.Iterable).Iterator()
	for it.HasNext() == types.True {
		// Modify the iter var in the fold activation.
		iterCtx.val = it.Next()

		// Evaluate the condition, terminate the loop if false.
		cond := fold.cond.Eval(iterCtx)
		condBool, ok := cond.(types.Bool)
		if !types.IsUnknown(cond) && ok && condBool != types.True {
			break
		}

		// Evalute the evaluation step into accu var.
		accuCtx.val = fold.step.Eval(iterCtx)
	}
	// Compute the result.
	res := fold.result.Eval(accuCtx)
	varActivationPool.Put(iterCtx)
	varActivationPool.Put(accuCtx)
	return res
}

// Optional Intepretable implementations that specialize, subsume, or extend the core evaluation
// plan via decorators.

// evalSetMembership is an Interpretable implementation which tests whether an input value
// exists within the set of map keys used to model a set.
type evalSetMembership struct {
	inst        Interpretable
	arg         Interpretable
	argTypeName string
	valueSet    map[ref.Val]ref.Val
}

// ID implements the Interpretable interface method.
func (e *evalSetMembership) ID() int64 {
	return e.inst.ID()
}

// Eval implements the Interpretable interface method.
func (e *evalSetMembership) Eval(vars Activation) ref.Val {
	val := e.arg.Eval(vars)
	if val.Type().TypeName() != e.argTypeName {
		return types.ValOrErr(val, "no such overload")
	}
	if ret, found := e.valueSet[val]; found {
		return ret
	}
	return types.False
}

// evalWatch is an Interpretable implementation that wraps the execution of a given
// expression so that it may observe the computed value and send it to an observer.
type evalWatch struct {
	inst     Interpretable
	observer evalObserver
}

// ID implements the Interpretable interface method.
func (e *evalWatch) ID() int64 {
	return e.inst.ID()
}

// Eval implements the Interpretable interface method.
func (e *evalWatch) Eval(vars Activation) ref.Val {
	val := e.inst.Eval(vars)
	e.observer(e.inst.ID(), val)
	return val
}

// evalWatchAttr describes a watcher of an attrInst interpretable.
//
// Since the watcher may be selected against at a later stage in program planning, the watcher
// must implement the attrInst interface by proxy.
type evalWatchAttr struct {
	inst     attrInst
	observer evalObserver
}

// ID implements the Interpretable interface method.
func (e *evalWatchAttr) ID() int64 {
	return e.inst.ID()
}

// Eval implements the Interpretable interface method.
func (e *evalWatchAttr) Eval(vars Activation) ref.Val {
	val := e.inst.Eval(vars)
	e.observer(e.inst.ID(), val)
	return val
}

func (e *evalWatchAttr) addField(id int64, name types.String) attrInst {
	return e.inst.addField(id, name)
}

func (e *evalWatchAttr) addIndex(pe *PathElem) attrInst {
	return e.inst.addIndex(pe)
}

func (e *evalWatchAttr) getAttrs() []Attribute {
	return e.inst.getAttrs()
}

func (e *evalWatchAttr) getAdapter() ref.TypeAdapter {
	return e.inst.getAdapter()
}

func (e *evalWatchAttr) getResolver() Resolver {
	return e.inst.getResolver()
}

// evalExhaustiveOr is just like evalOr, but does not short-circuit argument evaluation.
type evalExhaustiveOr struct {
	id  int64
	lhs Interpretable
	rhs Interpretable
}

// ID implements the Interpretable interface method.
func (or *evalExhaustiveOr) ID() int64 {
	return or.id
}

// Eval implements the Interpretable interface method.
func (or *evalExhaustiveOr) Eval(vars Activation) ref.Val {
	lVal := or.lhs.Eval(vars)
	rVal := or.rhs.Eval(vars)
	lBool, lok := lVal.(types.Bool)
	if lok && lBool == types.True {
		return types.True
	}
	rBool, rok := rVal.(types.Bool)
	if rok && rBool == types.True {
		return types.True
	}
	if lok && rok {
		return types.False
	}
	if types.IsUnknown(lVal) {
		return lVal
	}
	if types.IsUnknown(rVal) {
		return rVal
	}
	return types.ValOrErr(lVal, "no such overload")
}

// evalExhaustiveAnd is just like evalAnd, but does not short-circuit argument evaluation.
type evalExhaustiveAnd struct {
	id  int64
	lhs Interpretable
	rhs Interpretable
}

// ID implements the Interpretable interface method.
func (and *evalExhaustiveAnd) ID() int64 {
	return and.id
}

// Eval implements the Interpretable interface method.
func (and *evalExhaustiveAnd) Eval(vars Activation) ref.Val {
	lVal := and.lhs.Eval(vars)
	rVal := and.rhs.Eval(vars)
	lBool, lok := lVal.(types.Bool)
	if lok && lBool == types.False {
		return types.False
	}
	rBool, rok := rVal.(types.Bool)
	if rok && rBool == types.False {
		return types.False
	}
	if lok && rok {
		return types.True
	}
	if types.IsUnknown(lVal) {
		return lVal
	}
	if types.IsUnknown(rVal) {
		return rVal
	}
	return types.ValOrErr(lVal, "no such overload")
}

// evalExhaustiveConditional is like evalConditional, but does not short-circuit argument
// evaluation.
type evalExhaustiveConditional struct {
	id     int64
	expr   Interpretable
	truthy Interpretable
	falsy  Interpretable
}

// ID implements the Interpretable interface method.
func (cond *evalExhaustiveConditional) ID() int64 {
	return cond.id
}

// Eval implements the Interpretable interface method.
func (cond *evalExhaustiveConditional) Eval(vars Activation) ref.Val {
	cVal := cond.expr.Eval(vars)
	tVal := cond.truthy.Eval(vars)
	fVal := cond.falsy.Eval(vars)
	cBool, ok := cVal.(types.Bool)
	if !ok {
		return types.ValOrErr(cVal, "no such overload")
	}
	if cBool {
		return tVal
	}
	return fVal
}

// evalExhaustiveFold is like evalFold, but does not short-circuit argument evaluation.
type evalExhaustiveFold struct {
	id        int64
	accuVar   string
	iterVar   string
	iterRange Interpretable
	accu      Interpretable
	cond      Interpretable
	step      Interpretable
	result    Interpretable
}

// ID implements the Interpretable interface method.
func (fold *evalExhaustiveFold) ID() int64 {
	return fold.id
}

// Eval implements the Interpretable interface method.
func (fold *evalExhaustiveFold) Eval(vars Activation) ref.Val {
	foldRange := fold.iterRange.Eval(vars)
	if !foldRange.Type().HasTrait(traits.IterableType) {
		return types.ValOrErr(foldRange, "got '%T', expected iterable type", foldRange)
	}
	// Configure the fold activation with the accumulator initial value.
	accuCtx := varActivationPool.Get().(*varActivation)
	accuCtx.parent = vars
	accuCtx.name = fold.accuVar
	accuCtx.val = fold.accu.Eval(vars)
	iterCtx := varActivationPool.Get().(*varActivation)
	iterCtx.parent = accuCtx
	iterCtx.name = fold.iterVar
	it := foldRange.(traits.Iterable).Iterator()
	for it.HasNext() == types.True {
		// Modify the iter var in the fold activation.
		iterCtx.val = it.Next()

		// Evaluate the condition, but don't terminate the loop as this is exhaustive eval!
		fold.cond.Eval(iterCtx)

		// Evalute the evaluation step into accu var.
		accuCtx.val = fold.step.Eval(iterCtx)
	}
	// Compute the result.
	res := fold.result.Eval(accuCtx)
	varActivationPool.Put(iterCtx)
	varActivationPool.Put(accuCtx)
	return res
}

// attrInst is a private interface which is used to mark attribute evaluation steps.
//
// The attribute selection process varies significantly between checked and unchecked expressions
// as well as between straight field-selection and index operations on for absolute and relative
// attributes.
type attrInst interface {
	Interpretable

	// addField takes an expression id and string field to produce a more qualified attrInst
	addField(int64, types.String) attrInst

	// addIndex takes a PathElem value to produce a more qualified attrInst
	addIndex(*PathElem) attrInst

	// getAdapter returns the type adapter associated with the attrInst.
	getAdapter() ref.TypeAdapter

	// getResolver returns the Resolver associated with the attrInst.
	getResolver() Resolver

	// getAttrs returns the collection of Attribute values represented by this attrInst
	getAttrs() []Attribute
}

// evalAttr describes attribute evaluation on top-level variables within checked expressions.
type evalAttr struct {
	adapter  ref.TypeAdapter
	resolver Resolver
	attr     Attribute
}

// ID implements the Interpretable interface method.
func (e *evalAttr) ID() int64 {
	return e.attr.Variable().ID()
}

// Eval implements the Interpretable interface method.
func (e *evalAttr) Eval(vars Activation) ref.Val {
	// Attempt to resolve the attribute name.
	// When the variable cannot be located, the no such attribute error is returned.
	// When the variable exists, but a field selection does not resolve to a concrete value,
	// the expectation is that the Resolver will return an error.
	v, found := e.resolver.Resolve(vars, e.attr)
	if found {
		return e.adapter.NativeToValue(v)
	}
	return types.NewErr("no such attribute")
}

func (e *evalAttr) addField(id int64, name types.String) attrInst {
	// treat field and index selection the same way for absoluate attributes.
	return e.addIndex(newExprPathElem(id, name))
}

func (e *evalAttr) addIndex(pe *PathElem) attrInst {
	return &evalAttr{
		adapter:  e.adapter,
		attr:     e.attr.Select(pe),
		resolver: e.resolver,
	}
}

func (e *evalAttr) getAdapter() ref.TypeAdapter {
	return e.adapter
}

func (e *evalAttr) getResolver() Resolver {
	return e.resolver
}

func (e *evalAttr) getAttrs() []Attribute {
	return []Attribute{e.attr}
}

// evalOneofAttr is used to represent the possible Attribute values associated with
// an ident, select, or index operation within an unchecked expression.
type evalOneofAttr struct {
	id       int64
	adapter  ref.TypeAdapter
	resolver Resolver
	attrs    []Attribute
}

// ID implements the Interpretable interface method.
func (e *evalOneofAttr) ID() int64 {
	return e.id
}

// Eval implements the Interpretable interface method.
func (e *evalOneofAttr) Eval(vars Activation) ref.Val {
	for _, attr := range e.attrs {
		v, found := e.resolver.Resolve(vars, attr)
		if found {
			return e.adapter.NativeToValue(v)
		}
	}
	return types.NewErr("no such attribute")
}

func (e *evalOneofAttr) addField(id int64, name types.String) attrInst {
	// When a field is added to a oneof attribute, it should be appended to the name of the
	// top-most variable as a possible namespaced identifier, and then as a possible field
	// on all less-specified attributes.
	modAttrs := make([]Attribute, 0, len(e.attrs))
	pe := newExprPathElem(id, name)
	for _, attr := range e.attrs {
		if len(attr.Path()) != 0 {
			continue
		}
		v := attr.Variable()
		modName := fmt.Sprintf("%s.%s", v.Name(), name)
		modAttrs = append(modAttrs, newExprVarAttribute(id, modName))
	}
	for _, attr := range e.attrs {
		modAttrs = append(modAttrs, attr.Select(pe))
	}
	return &evalOneofAttr{
		id:       e.id,
		adapter:  e.adapter,
		attrs:    modAttrs,
		resolver: e.resolver,
	}
}

func (e *evalOneofAttr) addIndex(pe *PathElem) attrInst {
	// adding an index is not the same as adding a field since the index cannot possibly be an
	// identifier, so the selection behavior differs here.
	modAttrs := make([]Attribute, len(e.attrs), len(e.attrs))
	for i, attr := range e.attrs {
		modAttrs[i] = attr.Select(pe)
	}
	return &evalOneofAttr{
		id:       e.id,
		adapter:  e.adapter,
		attrs:    modAttrs,
		resolver: e.resolver,
	}
}

func (e *evalOneofAttr) getAdapter() ref.TypeAdapter {
	return e.adapter
}

func (e *evalOneofAttr) getResolver() Resolver {
	return e.resolver
}

func (e *evalOneofAttr) getAttrs() []Attribute {
	return e.attrs
}

// evalRelAttr specifies relative attribute selection against a computed value.
type evalRelAttr struct {
	id       int64
	adapter  ref.TypeAdapter
	resolver Resolver
	op       Interpretable
	attr     Attribute
}

// ID implements the Interpretable interface method.
func (e *evalRelAttr) ID() int64 {
	return e.id
}

// Eval implements the Interpretable interface method.
func (e *evalRelAttr) Eval(vars Activation) ref.Val {
	obj := e.op.Eval(vars)
	val := e.resolver.ResolveRelative(obj, vars, e.attr)
	return e.adapter.NativeToValue(val)
}

func (e *evalRelAttr) addField(id int64, name types.String) attrInst {
	// treat field selection as identical to index operations.
	return e.addIndex(newExprPathElem(id, name))
}

func (e *evalRelAttr) addIndex(pe *PathElem) attrInst {
	return &evalRelAttr{
		id:       e.id,
		adapter:  e.adapter,
		attr:     e.attr.Select(pe),
		op:       e.op,
		resolver: e.resolver,
	}
}

func (e *evalRelAttr) getAdapter() ref.TypeAdapter {
	return e.adapter
}

func (e *evalRelAttr) getResolver() Resolver {
	return e.resolver
}

func (e *evalRelAttr) getAttrs() []Attribute {
	return []Attribute{e.attr}
}

// evalConstEq describe the case where an attribute is compared against a constant value.
//
// When an evalConstEq is produced, this implies that the attribute value may be comparable in
// its Go native form without conversion first to a CEL representation.
//
// Note, the following implementation replicates some or all of the equality logic defined on a
// per-type basis within the CEL common types. This logic should be refactored to be shared
// between types and specialized comparison operations.
//
// TODO: replace the specialized equality with a general purpose invocation against a function
// table associated with the operand types.
type evalConstEq struct {
	id       int64
	attr     Attribute
	adapter  ref.TypeAdapter
	resolver Resolver
	val      ref.Val
}

// ID implements the Interpretable interface method.
func (e *evalConstEq) ID() int64 {
	return e.id
}

// Eval implements the Interpretable interface method.
func (e *evalConstEq) Eval(vars Activation) ref.Val {
	attrVal, found := e.resolver.Resolve(vars, e.attr)
	if !found {
		return types.NewErr("no such attribute")
	}
	return attrEqConst(attrVal, e.val, e.adapter)
}

// evalConstNe describe the case where an attribute is compared against a constant value.
//
// When an evalConstNe is produced, this implies that the attribute value may be comparable in
// its Go native form without conversion first to a CEL representation.
//
// Note, the following implementation replicates some or all of the equality logic defined on a
// per-type basis within the CEL common types. This logic should be refactored to be shared
// between types and specialized comparison operations.
//
// TODO: replace the specialized inequality with a general purpose invocation against a function
// table associated with the operand types.
type evalConstNe struct {
	id       int64
	attr     Attribute
	adapter  ref.TypeAdapter
	resolver Resolver
	val      ref.Val
}

func (e *evalConstNe) ID() int64 {
	return e.id
}

func (e *evalConstNe) Eval(vars Activation) ref.Val {
	attrVal, found := e.resolver.Resolve(vars, e.attr)
	if !found {
		return types.NewErr("no such attribute")
	}
	eq := attrEqConst(attrVal, e.val, e.adapter)
	eqBool, ok := eq.(types.Bool)
	if !ok {
		return eq
	}
	return !eqBool
}

func attrEqConst(attr interface{}, val ref.Val, adapter ref.TypeAdapter) ref.Val {
	switch attr.(type) {
	case ref.Val:
		return val.Equal(attr.(ref.Val))
	case string:
		attrStr := attr.(string)
		constStr, ok := val.(types.String)
		if !ok {
			return types.ValOrErr(val, "no such overload")
		}
		return types.Bool(attrStr == string(constStr))
	case float32:
		attrDbl := attr.(float32)
		constDbl, ok := val.(types.Double)
		if !ok {
			return types.ValOrErr(val, "no such overload")
		}
		return types.Bool(float64(attrDbl) == float64(constDbl))
	case float64:
		attrDbl := attr.(float64)
		constDbl, ok := val.(types.Double)
		if !ok {
			return types.ValOrErr(val, "no such overload")
		}
		return types.Bool(attrDbl == float64(constDbl))
	case int:
		attrInt := attr.(int)
		constInt, ok := val.(types.Int)
		if !ok {
			return types.ValOrErr(val, "no such overload")
		}
		return types.Bool(int64(attrInt) == int64(constInt))
	case int32:
		attrInt := attr.(int32)
		constInt, ok := val.(types.Int)
		if !ok {
			return types.ValOrErr(val, "no such overload")
		}
		return types.Bool(int64(attrInt) == int64(constInt))
	case int64:
		attrInt := attr.(int64)
		constInt, ok := val.(types.Int)
		if !ok {
			return types.ValOrErr(val, "no such overload")
		}
		return types.Bool(attrInt == int64(constInt))
	case bool:
		attrBool := attr.(bool)
		constBool, ok := val.(types.Bool)
		if !ok {
			return types.ValOrErr(val, "no such overload")
		}
		return types.Bool(attrBool == bool(constBool))
	case []string:
		attrListStr := attr.([]string)
		constList, ok := val.(traits.Lister)
		if !ok {
			return types.ValOrErr(val, "no such overload")
		}
		constListVal, ok := constList.Value().([]ref.Val)
		if !ok {
			return types.ValOrErr(val, "no such overload")
		}
		if len(constListVal) != len(attrListStr) {
			return types.False
		}
		for i, str := range attrListStr {
			v := constListVal[i]
			elemEq := attrEqConst(str, v, adapter)
			if elemEq != types.True {
				return elemEq
			}
		}
		return types.True
	default:
		attrVal := adapter.NativeToValue(attr)
		return attrVal.Equal(val)
	}
}
