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
	"strings"

	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// InterpretableDecorator is a functional interface for decorating or replacing
// Interpretable expression nodes at construction time.
type InterpretableDecorator func(Interpretable) (Interpretable, error)

// evalObserver is a functional interface that accepts an expression id and an observed value.
type evalObserver func(int64, ref.Val)

// decObserveEval records evaluation state into an EvalState object.
func decObserveEval(observer evalObserver) InterpretableDecorator {
	return func(i Interpretable) (Interpretable, error) {
		switch inst := i.(type) {
		case *evalWatch, *evalWatchAttr, *evalWatchConst:
			// these instruction are already watching, return straight-away.
			return i, nil
		case instAttr:
			return &evalWatchAttr{
				instAttr: inst,
				observer: observer,
			}, nil
		case instConst:
			return &evalWatchConst{
				instConst: inst,
				observer:  observer,
			}, nil
		default:
			return &evalWatch{
				inst:     i,
				observer: observer,
			}, nil
		}
	}
}

// decDisableShortcircuits ensures that all branches of an expression will be evaluated, no short-circuiting.
func decDisableShortcircuits() InterpretableDecorator {
	return func(i Interpretable) (Interpretable, error) {
		switch expr := i.(type) {
		case *evalOr:
			return &evalExhaustiveOr{
				id:  expr.id,
				lhs: expr.lhs,
				rhs: expr.rhs,
			}, nil
		case *evalAnd:
			return &evalExhaustiveAnd{
				id:  expr.id,
				lhs: expr.lhs,
				rhs: expr.rhs,
			}, nil
		case *evalFold:
			return &evalExhaustiveFold{
				id:        expr.id,
				accu:      expr.accu,
				accuVar:   expr.accuVar,
				iterRange: expr.iterRange,
				iterVar:   expr.iterVar,
				cond:      expr.cond,
				step:      expr.step,
				result:    expr.result,
			}, nil
		case instAttr:
			cond, isCond := expr.Attr().(*conditionalAttribute)
			if isCond {
				return &evalExhaustiveConditional{
					id:      cond.id,
					attr:    cond,
					adapter: expr.Adapter(),
				}, nil
			}
		}
		return i, nil
	}
}

// decOptimize optimizes the program plan by looking for common evaluation patterns and
// conditionally precomputating the result.
// - build list and map values with constant elements.
// - convert 'in' operations to set membership tests if possible.
func decOptimize() InterpretableDecorator {
	return func(i Interpretable) (Interpretable, error) {
		switch expr := i.(type) {
		case *evalList:
			return maybeBuildListLiteral(i, expr)
		case *evalMap:
			return maybeBuildMapLiteral(i, expr)
		case *evalBinary:
			if expr.overload == overloads.InList {
				return maybeOptimizeSetMembership(i, expr)
			}
		}
		return i, nil
	}
}

func maybeBuildListLiteral(i Interpretable, l *evalList) (Interpretable, error) {
	for _, elem := range l.elems {
		_, isConst := elem.(*evalConst)
		if !isConst {
			return i, nil
		}
	}
	val := l.Eval(EmptyActivation())
	return &evalConst{
		id:  l.id,
		val: val,
	}, nil
}

func maybeBuildMapLiteral(i Interpretable, mp *evalMap) (Interpretable, error) {
	for idx, key := range mp.keys {
		_, isConst := key.(*evalConst)
		if !isConst {
			return i, nil
		}
		_, isConst = mp.vals[idx].(*evalConst)
		if !isConst {
			return i, nil
		}
	}
	val := mp.Eval(EmptyActivation())
	return &evalConst{
		id:  mp.id,
		val: val,
	}, nil
}

// maybeOptimizeSetMembership may convert an 'in' operation against a list to map key membership
// test if the following conditions are true:
// - the list is a constant with homogeneous element types.
// - the elements are all of primitive type.
func maybeOptimizeSetMembership(i Interpretable, inlist *evalBinary) (Interpretable, error) {
	l, isConst := inlist.rhs.(*evalConst)
	if !isConst {
		return i, nil
	}
	// When the incoming binary call is flagged with as the InList overload, the value will
	// always be convertible to a `traits.Lister` type.
	list := l.val.(traits.Lister)
	if list.Size() == types.IntZero {
		return &evalConst{
			id:  inlist.id,
			val: types.False,
		}, nil
	}
	it := list.Iterator()
	var typ ref.Type
	valueSet := make(map[ref.Val]ref.Val)
	for it.HasNext() == types.True {
		elem := it.Next()
		if !types.IsPrimitiveType(elem) {
			// Note, non-primitive type are not yet supported.
			return i, nil
		}
		if typ == nil {
			typ = elem.Type()
		} else if typ.TypeName() != elem.Type().TypeName() {
			return i, nil
		}
		valueSet[elem] = types.True
	}
	return &evalSetMembership{
		inst:        inlist,
		arg:         inlist.lhs,
		argTypeName: typ.TypeName(),
		valueSet:    valueSet,
	}, nil
}

type maybeNativeOverload func(Interpretable) (Interpretable, error)

func isAttrOnlyBinary(call Interpretable) bool {
	bin, ok := call.(*evalBinary)
	if !ok {
		return false
	}
	_, lhsIsAttr := bin.lhs.(instAttr)
	_, rhsIsAttr := bin.rhs.(instAttr)
	return lhsIsAttr && rhsIsAttr
}

func isAttrAndConstBinary(call Interpretable) bool {
	bin, ok := call.(*evalBinary)
	if !ok {
		return false
	}
	_, lhsIsAttr := bin.lhs.(instAttr)
	_, lhsIsConst := bin.rhs.(instAttr)
	_, rhsIsAttr := bin.rhs.(instAttr)
	_, rhsIsConst := bin.rhs.(instAttr)
	return lhsIsAttr && rhsIsConst || lhsIsConst && rhsIsAttr
}

var nativeOverloads = map[string]maybeNativeOverload{
	overloads.Equals: func(call Interpretable) (Interpretable, error) {
		if isAttrAndConstBinary(call) {

		}
		return call, nil
	},
	overloads.NotEquals: func(call Interpretable) (Interpretable, error) {
		if isAttrAndConstBinary(call) {

		}
		return call, nil
	},
	overloads.EndsWithString: func(call Interpretable) (Interpretable, error) {
		if isAttrOnlyBinary(call) {

		}
		if isAttrAndConstBinary(call) {

		}
		return call, nil
	},
	overloads.StartsWithString: func(call Interpretable) (Interpretable, error) {
		if isAttrOnlyBinary(call) {
			bin := call.(*evalBinary)
			return &evalBinaryAttrNative{
				id:      bin.id,
				lhs:     bin.lhs.(instAttr).Attr(),
				rhs:     bin.rhs.(instAttr).Attr(),
				fun:     strStartsWith,
				adapter: bin.lhs.(instAttr).Adapter(),
			}, nil
		}
		if isAttrAndConstBinary(call) {
			var arg instAttr
			var val ref.Val
			bin := call.(*evalBinary)
			return &evalBinaryAttrConstNative{
				id:      bin.id,
				arg:     arg.Attr(),
				val:     val,
				fun:     strStartsWith,
				adapter: arg.Adapter(),
			}, nil
		}
		return call, nil
	},
}

type evalBinaryAttrNative struct {
	id      int64
	lhs     Attribute
	rhs     Attribute
	fun     func(lhs, rhs interface{}) (interface{}, error)
	adapter ref.TypeAdapter
}

func (e *evalBinaryAttrNative) ID() int64 {
	return e.id
}

func (e *evalBinaryAttrNative) Eval(ctx Activation) ref.Val {
	l, err := e.lhs.Resolve(ctx)
	if err != nil {
		return types.NewErr(err.Error())
	}
	lUnk, ok := l.(types.Unknown)
	if ok {
		return lUnk
	}
	r, err := e.rhs.Resolve(ctx)
	if err != nil {
		return types.NewErr(err.Error())
	}
	rUnk, ok := r.(types.Unknown)
	if ok {
		return rUnk
	}
	v, err := e.fun(l, r)
	if err != nil {
		return types.NewErr(err.Error())
	}
	return e.adapter.NativeToValue(v)
}

type evalBinaryAttrConstNative struct {
	id      int64
	arg     Attribute
	val     ref.Val
	fun     func(lhs, rhs interface{}) (interface{}, error)
	adapter ref.TypeAdapter
}

func (e *evalBinaryAttrConstNative) ID() int64 {
	return e.id
}

func (e *evalBinaryAttrConstNative) Eval(ctx Activation) ref.Val {
	arg, err := e.arg.Resolve(ctx)
	if err != nil {
		return types.NewErr(err.Error())
	}
	unk, ok := arg.(types.Unknown)
	if ok {
		return unk
	}
	v, err := e.fun(arg, e.val.Value())
	if err != nil {
		return types.NewErr(err.Error())
	}
	return e.adapter.NativeToValue(v)
}

func strEndsWith(str, suffix interface{}) (interface{}, error) {
	var s, suf string
	switch v := str.(type) {
	case string:
		s = v
	case types.String:
		s = string(v)
	default:
		return nil, fmt.Errorf("no such overload")
	}

	switch v := suffix.(type) {
	case string:
		suf = v
	case types.String:
		suf = string(v)
	default:
		return nil, fmt.Errorf("no such overload")
	}
	return strings.HasSuffix(s, suf), nil
}

func strStartsWith(str, prefix interface{}) (interface{}, error) {
	var s, pre string
	switch v := str.(type) {
	case string:
		s = v
	case types.String:
		s = string(v)
	default:
		return nil, fmt.Errorf("no such overload")
	}

	switch v := prefix.(type) {
	case string:
		pre = v
	case types.String:
		pre = string(v)
	default:
		return nil, fmt.Errorf("no such overload")
	}
	return strings.HasPrefix(s, pre), nil
}
