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

// Uint type implementation which supports comparison and math operators.
type Uint uint64

var (
	// UintType singleton.
	UintType = NewTypeValue("uint",
		traits.AdderType,
		traits.ComparerType,
		traits.DividerType,
		traits.ModderType,
		traits.MultiplierType,
		traits.SubtractorType)
)

const (
	uintZero = Uint(0)
)

func (i Uint) Add(other ref.Value) ref.Value {
	if UintType != other.Type() {
		return NewErr("unsupported overload")
	}
	return i + other.(Uint)
}

func (i Uint) Compare(other ref.Value) ref.Value {
	if UintType != other.Type() {
		return NewErr("unsupported overload")
	}
	if i < other.(Uint) {
		return IntNegOne
	}
	if i > other.(Uint) {
		return IntOne
	}
	return IntZero
}

func (i Uint) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	refKind := typeDesc.Kind()
	switch refKind {
	case reflect.Uint32:
		return uint32(i), nil
	case reflect.Uint64:
		return uint64(i), nil
	case reflect.Ptr:
		if typeDesc == jsonValueType {
			return &structpb.Value{
				Kind: &structpb.Value_NumberValue{
					NumberValue: float64(i)}}, nil
		}
		switch typeDesc.Elem().Kind() {
		case reflect.Uint32:
			ptr := uint32(i)
			return &ptr, nil
		case reflect.Uint64:
			ptr := uint64(i)
			return &ptr, nil
		}
	case reflect.Interface:
		if reflect.TypeOf(i).Implements(typeDesc) {
			return i, nil
		}
	}

	return nil, fmt.Errorf("unsupported type conversion from 'uint' to %v", typeDesc)
}

func (i Uint) ConvertToType(typeVal ref.Type) ref.Value {
	switch typeVal {
	case IntType:
		return Int(i)
	case UintType:
		return i
	case DoubleType:
		return Double(i)
	case StringType:
		return String(fmt.Sprintf("%d", uint64(i)))
	case TypeType:
		return UintType
	}
	return NewErr("type conversion error from '%s' to '%s'", UintType, typeVal)
}

func (i Uint) Divide(other ref.Value) ref.Value {
	if UintType != other.Type() {
		return NewErr("unsupported overload")
	}
	otherUint := other.(Uint)
	if otherUint == uintZero {
		return NewErr("divide by zero")
	}
	return i / otherUint
}

func (i Uint) Equal(other ref.Value) ref.Value {
	return Bool(UintType == other.Type() &&
		i.Value() == other.Value())
}

func (i Uint) Modulo(other ref.Value) ref.Value {
	if UintType != other.Type() {
		return NewErr("unsupported overload")
	}
	otherUint := other.(Uint)
	if otherUint == uintZero {
		return NewErr("modulus by zero")
	}
	return i % otherUint
}

func (i Uint) Multiply(other ref.Value) ref.Value {
	if UintType != other.Type() {
		return NewErr("unsupported overload")
	}
	return i * other.(Uint)
}

func (i Uint) Subtract(subtrahend ref.Value) ref.Value {
	if UintType != subtrahend.Type() {
		return NewErr("unsupported overload")
	}
	return i - subtrahend.(Uint)
}

func (i Uint) Type() ref.Type {
	return UintType
}

func (i Uint) Value() interface{} {
	return uint64(i)
}
