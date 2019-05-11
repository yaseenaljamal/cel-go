package interpreter

import (
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
	Eval(Activation) ref.Val
}

type evalSelect struct {
	id        int64
	op        Interpretable
	field     types.String
	resolveID func(Activation) (ref.Val, bool)
}

// ID implements the Interpretable interface method.
func (sel *evalSelect) ID() int64 {
	return sel.id
}

// Eval implements the Interpretable interface method.
func (sel *evalSelect) Eval(ctx Activation) ref.Val {
	obj := sel.op.Eval(ctx)
	indexer, ok := obj.(traits.Indexer)
	if !ok {
		return types.ValOrErr(obj, "invalid type for field selection.")
	}
	return indexer.Get(sel.field)
}

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
func (test *evalTestOnly) Eval(ctx Activation) ref.Val {
	obj := test.op.Eval(ctx)
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
func (cons *evalConst) Eval(ctx Activation) ref.Val {
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
func (or *evalOr) Eval(ctx Activation) ref.Val {
	// short-circuit lhs.
	lVal := or.lhs.Eval(ctx)
	lBool, lok := lVal.(types.Bool)
	if lok && lBool == types.True {
		return types.True
	}
	// short-circuit on rhs.
	rVal := or.rhs.Eval(ctx)
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
func (and *evalAnd) Eval(ctx Activation) ref.Val {
	// short-circuit lhs.
	lVal := and.lhs.Eval(ctx)
	lBool, lok := lVal.(types.Bool)
	if lok && lBool == types.False {
		return types.False
	}
	// short-circuit on rhs.
	rVal := and.rhs.Eval(ctx)
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
func (cond *evalConditional) Eval(ctx Activation) ref.Val {
	condVal := cond.expr.Eval(ctx)
	condBool, ok := condVal.(types.Bool)
	if !ok {
		return types.ValOrErr(condVal, "no such overload")
	}
	if condBool {
		return cond.truthy.Eval(ctx)
	}
	return cond.falsy.Eval(ctx)
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
func (eq *evalEq) Eval(ctx Activation) ref.Val {
	lVal := eq.lhs.Eval(ctx)
	rVal := eq.rhs.Eval(ctx)
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
func (ne *evalNe) Eval(ctx Activation) ref.Val {
	lVal := ne.lhs.Eval(ctx)
	rVal := ne.rhs.Eval(ctx)
	eqVal := lVal.Equal(rVal)
	eqBool, ok := eqVal.(types.Bool)
	if !ok {
		if types.IsUnknown(eqVal) {
			return eqVal
		}
		return types.NewErr("no such overload: _!=_")
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
func (zero *evalZeroArity) Eval(ctx Activation) ref.Val {
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
func (un *evalUnary) Eval(ctx Activation) ref.Val {
	argVal := un.arg.Eval(ctx)
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
func (bin *evalBinary) Eval(ctx Activation) ref.Val {
	lVal := bin.lhs.Eval(ctx)
	rVal := bin.rhs.Eval(ctx)
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
		rcvr := lVal.(traits.Receiver)
		return rcvr.ReceiveBinary(
			bin.function,
			bin.overload,
			rVal)
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
func (fn *evalVarArgs) Eval(ctx Activation) ref.Val {
	argVals := make([]ref.Val, len(fn.args), len(fn.args))
	// Early return if any argument to the function is unknown or error.
	for i, arg := range fn.args {
		argVals[i] = arg.Eval(ctx)
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
		return arg0.(traits.Receiver).Receive(fn.function, fn.overload, argVals[1:]...)
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
func (l *evalList) Eval(ctx Activation) ref.Val {
	elemVals := make([]ref.Val, len(l.elems), len(l.elems))
	// If any argument is unknown or error early terminate.
	for i, elem := range l.elems {
		elemVal := elem.Eval(ctx)
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
func (m *evalMap) Eval(ctx Activation) ref.Val {
	entries := make(map[ref.Val]ref.Val)
	// If any argument is unknown or error early terminate.
	for i, key := range m.keys {
		keyVal := key.Eval(ctx)
		if types.IsUnknownOrError(keyVal) {
			return keyVal
		}
		valVal := m.vals[i].Eval(ctx)
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
func (o *evalObj) Eval(ctx Activation) ref.Val {
	fieldVals := make(map[string]ref.Val)
	// If any argument is unknown or error early terminate.
	for i, field := range o.fields {
		val := o.vals[i].Eval(ctx)
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
func (fold *evalFold) Eval(ctx Activation) ref.Val {
	foldRange := fold.iterRange.Eval(ctx)
	if !foldRange.Type().HasTrait(traits.IterableType) {
		return types.ValOrErr(foldRange, "got '%T', expected iterable type", foldRange)
	}
	// Configure the fold activation with the accumulator initial value.
	accuCtx := varActivationPool.Get().(*varActivation)
	accuCtx.parent = ctx
	accuCtx.name = fold.accuVar
	accuCtx.val = fold.accu.Eval(ctx)
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

var (
	// no arg value for unary receiver methods.
	noArg = []ref.Val{}
)
