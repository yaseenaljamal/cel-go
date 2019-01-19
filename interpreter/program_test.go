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
	"testing"

	"github.com/google/cel-go/test"
)

func TestNewExhaustiveProgram_LogicalOr(t *testing.T) {
	program, _ := NewProgram(
		test.LogicalOr.Expr,
		test.LogicalOr.Info(t.Name()),
		ExhaustiveProgram(true))
	if loc, found := program.Metadata().IDLocation(1); found {
		t.Errorf("Unexpected location found: %v", loc)
	}
	state := newEvalState(program.MaxInstructionID() + 1)
	program.plan(state)
	fmt.Printf("%s\n%s\n\n", t.Name(), program)

	expected := "TestNewExhaustiveProgram_LogicalOr\n" +
		"0: mov   map(map[6:7]), r5\n" +
		"1: call  select(5, 'c'), r8\n" +
		"2: local 'a', r1\n" +
		"3: call  _||_(r8, r1), r2\n\n"
	actual := fmt.Sprintf("%s\n%s\n\n", t.Name(), program)
	if actual != expected {
		t.Errorf("program did not compile as expected. actual: %v\nexpected: %v",
			actual, expected)
	}
}

func TestNewExhaustiveProgram_Conditional(t *testing.T) {
	program, _ := NewProgram(
		test.Conditional.Expr,
		test.Conditional.Info(t.Name()),
		ExhaustiveProgram(true))
	if loc, found := program.Metadata().IDLocation(1); found {
		t.Errorf("Unexpected location found: %v", loc)
	}
	state := newEvalState(program.MaxInstructionID() + 1)
	program.plan(state)
	expected := "TestNewExhaustiveProgram_Conditional\n" +
		"0: local 'a', r1\n" +
		"1: local 'b', r2\n" +
		"2: call  _<_(r2, r4), r3\n" +
		"3: local 'c', r5\n" +
		"4: mov   list([7]), r8\n" +
		"5: call  _==_(r5, r8), r6\n" +
		"6: call  _?_:_(r1, r3, r6), r9\n\n"
	actual := fmt.Sprintf("%s\n%s\n\n", t.Name(), program)
	if actual != expected {
		t.Errorf("program did not compile as expected. actual: %v\nexpected: %v",
			actual, expected)
	}
}

func TestNewProgram_Empty(t *testing.T) {
	program, _ := NewProgram(
		test.Empty.Expr,
		test.Empty.Info(t.Name()))
	if loc, found := program.Metadata().IDLocation(0); found {
		t.Errorf("Unexpected location found: %v", loc)
	}
	state := newEvalState(program.MaxInstructionID() + 1)
	program.plan(state)
}

func TestNewProgram_LogicalAnd(t *testing.T) {
	program, _ := NewProgram(
		test.LogicalAnd.Expr,
		test.LogicalAnd.Info(t.Name()))
	if loc, found := program.Metadata().IDLocation(1); found {
		t.Errorf("Unexpected location found: %v", loc)
	}
	state := newEvalState(program.MaxInstructionID() + 1)
	program.plan(state)
	fmt.Printf("%s\n%s\n\n", t.Name(), program)
}

func TestNewProgram_LogicalOr(t *testing.T) {
	program, _ := NewProgram(
		test.LogicalOr.Expr,
		test.LogicalOr.Info(t.Name()))
	if loc, found := program.Metadata().IDLocation(1); found {
		t.Errorf("Unexpected location found: %v", loc)
	}
	state := newEvalState(program.MaxInstructionID() + 1)
	program.plan(state)
	//  "{c: false}.c || a"
	expected := `TestNewProgram_LogicalOr
0: mov   map(map[6:7]), r5
1: call  select(5, 'c'), r8
2: jump  3 if cond<r8>
3: local 'a', r1
4: call  _||_(r8, r1), r2`
	actual := fmt.Sprintf("%s\n%s", t.Name(), program)
	if actual != expected {
		t.Errorf("Got '%s', wanted '%s'", actual, expected)
	}
}

func TestNewProgram_Conditional(t *testing.T) {
	program, _ := NewProgram(
		test.Conditional.Expr,
		test.Conditional.Info(t.Name()))
	if loc, found := program.Metadata().IDLocation(1); found {
		t.Errorf("Unexpected location found: %v", loc)
	}
	state := newEvalState(program.MaxInstructionID() + 1)
	program.plan(state)
	expected := "TestNewProgram_Conditional\n" +
		"0: local 'a', r1\n" +
		"1: jump  8 if cond<r1>\n" +
		"2: jump  4 if cond<r1>\n" +
		"3: local 'b', r2\n" +
		"4: call  _<_(r2, r4), r3\n" +
		"5: jump  4 if cond<r3>\n" +
		"6: local 'c', r5\n" +
		"7: mov   list([7]), r8\n" +
		"8: call  _==_(r5, r8), r6\n" +
		"9: call  _?_:_(r1, r3, r6), r9\n\n"
	actual := fmt.Sprintf("%s\n%s\n\n", t.Name(), program)
	if actual != expected {
		t.Errorf("program did not compile as expected. actual: %v\nexpected: %v",
			actual, expected)
	}
}

func TestNewProgram_Comprehension(t *testing.T) {
	program, _ := NewProgram(
		test.Exists.Expr,
		test.Exists.Info(t.Name()))
	if loc, found := program.Metadata().IDLocation(1); !found {
		t.Errorf("Unexpected location found: %v", loc)
	}
	state := newEvalState(program.MaxInstructionID() + 1)
	program.plan(state)
	fmt.Printf("%s\n%s\n\n", t.Name(), program)
}

func TestNewProgram_DynMap(t *testing.T) {
	program, _ := NewProgram(
		test.DynMap.Expr,
		test.DynMap.Info(t.Name()))
	if loc, found := program.Metadata().IDLocation(1); found {
		t.Errorf("Unexpected location found: %v", loc)
	}
	state := newEvalState(program.MaxInstructionID() + 1)
	program.plan(state)
	fmt.Printf("%s\n%s\n\n", t.Name(), program)
}
