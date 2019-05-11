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

/*
import (
	"testing"

	"github.com/google/cel-go/common/types"
)

func TestNewActivation(t *testing.T) {
	activation, _ := NewActivation(map[string]interface{}{"a": types.True})
	if val, found := activation.ResolveName("a"); !found || val != types.True {
		t.Error("Activation failed to resolve 'a'")
	}
}

func TestHierarchicalActivation(t *testing.T) {
	// compose a parent with more properties than the child
	parent, _ := NewActivation(map[string]interface{}{
		"a": types.String("world"),
		"b": types.Int(-42),
	})
	// compose the child such that it shadows the parent
	child, _ := NewActivation(map[string]interface{}{
		"a": types.True,
		"c": types.String("universe"),
	})
	combined := NewHierarchicalActivation(parent, child)

	// Resolve the shadowed child value.
	if val, found := combined.ResolveName("a"); !found || val != types.True {
		t.Error("Activation failed to resolve shadow value of 'a'")
	}
	// Resolve the parent only value.
	if val, found := combined.ResolveName("b"); !found || val.(types.Int) != -42 {
		t.Error("Activation failed to resolve parent value of 'b'")
	}
	// Resolve the child only value.
	if val, found := combined.ResolveName("c"); !found || val.(types.String) != "universe" {
		t.Error("Activation failed to resolve child value of 'c'")
	}
}

func TestActivation_NilValue(t *testing.T) {
	var ptr *string
	var fun *func()
	a, _ := NewActivation(map[string]interface{}{
		"nil": nil,  // plain old nil
		"ptr": ptr,  // nil pointer to a supported type
		"fun": fun}) // nil pointer to an unknown type
	if v, found := a.ResolveName("nil"); !found || v != types.NullValue {
		t.Errorf("Got '%v', wanted 'null'", v)
	}
	if v, _ := a.ResolveName("ptr"); !types.IsError(v) {
		t.Errorf("Got '%v', wanted error", v)
	}
	if v, _ := a.ResolveName("fun"); !types.IsError(v) {
		t.Errorf("Got '%v', wanted error", v)
	}
}

func TestAdaptingActivation_NilValue(t *testing.T) {
	var ptr *types.String
	a, _ := NewAdaptingActivation(types.NewRegistry(), map[string]interface{}{
		"nil": nil,
		"ptr": ptr})
	if v, found := a.ResolveName("nil"); !found || v != types.NullValue {
		t.Errorf("Got '%v', wanted 'null'", v)
	}
	if v, _ := a.ResolveName("ptr"); !types.IsError(v) {
		t.Errorf("Got '%v', wanted error", v)
	}
	if v, found := a.ResolveName("missing"); found {
		t.Errorf("Got '%v', wanted not found", v)
	}
}
*/