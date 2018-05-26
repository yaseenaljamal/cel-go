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

package types

import (
	"fmt"
	"github.com/golang/protobuf/ptypes/struct"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"reflect"
)

// Int type that implements ref.Value as well as comparison and math operators.
type Int int64

const (
	// Int constants used for comparison results.
	IntZero   = Int(0)
	IntOne    = Int(1)
	IntNegOne = Int(-1)
)

var (
	// IntType singleton.
	IntType = NewTypeValue("int",
		traits.AdderType,
		traits.ComparerType,
		traits.DividerType,
		traits.ModderType,
		traits.MultiplierType,
		traits.NegatorType,
		traits.SubtractorType)
)

func (i Int) Add(other ref.Value) ref.Value {
	if IntType != other.Type() {
		return NewErr("unsupported overload")
	}
	return i + other.(Int)
}

func (i Int) Compare(other ref.Value) ref.Value {
	if IntType != other.Type() {
		return NewErr("unsupported overload")
	}
	if i < other.(Int) {
		return IntNegOne
	}
	if i > other.(Int) {
		return IntOne
	}
	return IntZero
}

func (i Int) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	refKind := typeDesc.Kind()
	switch refKind {
	case reflect.Int32:
		return int32(i), nil
	case reflect.Int64:
		return int64(i), nil
	case reflect.Ptr:
		if typeDesc == jsonValueType {
			return &structpb.Value{
				Kind: &structpb.Value_NumberValue{
					NumberValue: float64(i)}}, nil
		}
		switch typeDesc.Elem().Kind() {
		case reflect.Int32:
			ptr := int32(i)
			return &ptr, nil
		case reflect.Int64:
			ptr := int64(i)
			return &ptr, nil
		}
	case reflect.Interface:
		if reflect.TypeOf(i).Implements(typeDesc) {
			return i, nil
		}
	}
	return nil, fmt.Errorf("unsupported type conversion from 'int' to %v", typeDesc)
}

func (i Int) ConvertToType(typeVal ref.Type) ref.Value {
	switch typeVal {
	case IntType:
		return i
	case UintType:
		return Uint(i)
	case DoubleType:
		return Double(i)
	case StringType:
		return String(fmt.Sprintf("%d", int64(i)))
	case TypeType:
		return IntType
	}
	return NewErr("type conversion error from '%s' to '%s'", IntType, typeVal)
}

func (i Int) Divide(other ref.Value) ref.Value {
	if IntType != other.Type() {
		return NewErr("unsupported overload")
	}
	otherInt := other.(Int)
	if otherInt == IntZero {
		return NewErr("divide by zero")
	}
	return i / otherInt
}

func (i Int) Equal(other ref.Value) ref.Value {
	return Bool(IntType == other.Type() && i.Value() == other.Value())
}

func (i Int) Modulo(other ref.Value) ref.Value {
	if IntType != other.Type() {
		return NewErr("unsupported overload")
	}
	otherInt := other.(Int)
	if otherInt == IntZero {
		return NewErr("modulus by zero")
	}
	return i % otherInt
}

func (i Int) Multiply(other ref.Value) ref.Value {
	if IntType != other.Type() {
		return NewErr("unsupported overload")
	}
	return i * other.(Int)
}

func (i Int) Negate() ref.Value {
	return -i
}

func (i Int) Subtract(subtrahend ref.Value) ref.Value {
	if IntType != subtrahend.Type() {
		return NewErr("unsupported overload")
	}
	return i - subtrahend.(Int)
}

func (i Int) Type() ref.Type {
	return IntType
}

func (i Int) Value() interface{} {
	return int64(i)
}
