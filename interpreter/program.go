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

	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/overloads"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// programFlag represents a program execution mode which is optionally enabled.
type programFlag int64

const (
	// programFlagTrackState will ensure that a hermetic evaluation state is returned on
	// eval completion.
	programFlagTrackState programFlag = 1

	// programFlagNoShortCircuit disables short-circuit jumps to ensure all branches are
	// evaluated during program execution.
	programFlagNoShortCircuit programFlag = 2

	// programFlagExhaustive combines the state tracking and no-short-curcuit flags.
	programFlagExhaustive programFlag = programFlagTrackState |
		programFlagNoShortCircuit
)

// Program contains instructions and related metadata.
type Program struct {
	expression      *exprpb.Expr
	Instructions    []Instruction
	metadata        Metadata
	refMap          map[int64]*exprpb.Reference
	revInstructions map[int64]int
	flags           programFlag
	resultID        int64
}

// NewCheckedProgram creates a Program from a checked CEL expression.
func NewCheckedProgram(c *exprpb.CheckedExpr) *Program {
	// TODO: take advantage of the type-check information.
	return NewProgram(c.Expr, c.SourceInfo)
}

// NewProgram creates a Program from a CEL expression and source information.
func NewProgram(expression *exprpb.Expr,
	info *exprpb.SourceInfo) *Program {
	revInstructions := make(map[int64]int)
	return &Program{
		expression:      expression,
		revInstructions: revInstructions,
		metadata:        newExprMetadata(info),
	}
}

// NewExhaustiveProgram creates a Program from a CEL expression and source
// information which force evaluating all branches of the expression.
func NewExhaustiveProgram(expression *exprpb.Expr,
	// TODO: also disable short circuit in comprehensions.
	info *exprpb.SourceInfo) *Program {
	revInstructions := make(map[int64]int)
	return &Program{
		expression:      expression,
		revInstructions: revInstructions,
		metadata:        newExprMetadata(info),
		flags:           programFlagExhaustive,
	}
}

// GetInstruction returns the instruction at the given runtime expression id.
func (p *Program) GetInstruction(runtimeID int64) Instruction {
	return p.Instructions[p.revInstructions[runtimeID]]
}

// plan ensures that instructions have been properly initialized prior to beginning the execution
// of a program. The plan step may optimize the instruction set if it chooses.
func (p *Program) plan(state *evalState) {
	if p.Instructions == nil {
		shortcircuit := p.flags&programFlagNoShortCircuit == 0
		p.resultID = p.expression.Id
		p.Instructions = WalkExpr(
			p.expression,
			p.metadata,
			state,
			shortcircuit)
		for i, inst := range p.Instructions {
			p.revInstructions[inst.GetID()] = i
		}
	}
}

// bindOperators iterates over the instruction set attempting to resolve function
// names to implementation references within the instruction set.
func (p *Program) bindOperators(disp Dispatcher) {
	if p.Instructions != nil {
		for _, inst := range p.Instructions {
			call, found := inst.(*CallExpr)
			if !found {
				continue
			}
			fn, found := disp.FindOverload(call.Function)
			if !found {
				continue
			}
			call.Impl = fn
		}
	}
}

// computeConstExprs iterates over the instruction set attempting to resolve function
// names to implementation references within the instruction set.
func (p *Program) computeConstExprs(state *evalState) {
	instructions := p.Instructions
	if instructions == nil {
		return
	}
	// Accumulate a list of instructions which are now constant values.
	var constInsts []int
	var nested int
	for i, inst := range instructions {
		switch inst.(type) {
		case *PushScopeInst:
			nested++
		case *PopScopeInst:
			nested--
		case *CallExpr:
			if nested != 0 {
				continue
			}
			call := inst.(*CallExpr)
			if p.maybeComputeConstCall(call, state) {
				switch call.Function {
				case operators.LogicalAnd, operators.LogicalOr:
					// remove the jump condition when both args are constant.
					constInsts = append(constInsts, i-1)
				}
				constInsts = append(constInsts, i)
			}
		}
	}
	// If no constant expressions were encountered, return.
	constInstsCnt := len(constInsts)
	if constInstsCnt == 0 {
		return
	}

	// Otherwise, copy non-const instructions into an new list.
	optInstsCnt := len(instructions) - constInstsCnt
	optInsts := make([]Instruction, optInstsCnt, optInstsCnt)
	i := 0
	oi := 0
	for _, j := range constInsts {
		for i < j {
			optInsts[oi] = instructions[i]
			oi++
			i++
		}
		i = j + 1
	}
	for oi < optInstsCnt {
		optInsts[oi] = instructions[i]
		oi++
		i++
	}
	p.Instructions = optInsts
	// Iterate until there are no more const expressions left.
	p.computeConstExprs(state)
}

func (p *Program) maybeComputeConstCall(call *CallExpr, state *evalState) bool {
	// Skip functions without an implementation.
	if call.Impl == nil {
		return false
	}
	switch call.Function {
	case operators.NotStrictlyFalse,
		operators.OldNotStrictlyFalse,
		overloads.Iterator,
		overloads.HasNext,
		overloads.Next:
		return false
	}
	switch len(call.Args) {
	case 1:
		arg0 := state.values[call.Args[0]]
		if arg0 == nil {
			return false
		}
		if call.Impl.OperandTrait != 0 && !arg0.Type().HasTrait(call.Impl.OperandTrait) {
			return false
		}
		state.values[call.ID] = call.Impl.Unary(arg0)
		return true
	case 2:
		arg0 := state.values[call.Args[0]]
		arg1 := state.values[call.Args[1]]
		if arg0 == nil || arg1 == nil {
			return false
		}
		if call.Impl.OperandTrait != 0 && !arg0.Type().HasTrait(call.Impl.OperandTrait) {
			return false
		}
		state.values[call.ID] = call.Impl.Binary(arg0, arg1)
		return true
	}
	return false
}

// MaxInstructionID returns the identifier of the last expression in the
// program.
func (p *Program) MaxInstructionID() int64 {
	// The max instruction id is the highest expression id in the program,
	// plus the count of the internal variables allocated for comprehensions.
	//
	// A comprehension allocates an id for each of the following:
	// - iterator
	// - hasNext() result
	// - iterVar
	//
	// The maxID is thus, the max input id + comprehension count * 3
	return maxID(p.expression) + comprehensionCount(p.expression)*3
}

// Metadata used to determine source locations of sub-expressions.
func (p *Program) Metadata() Metadata {
	return p.metadata
}

func (p *Program) String() string {
	instStrs := make([]string, len(p.Instructions), len(p.Instructions))
	for i, inst := range p.Instructions {
		instStrs[i] = fmt.Sprintf("%d: %v", i, inst)
	}
	return strings.Join(instStrs, "\n")
}

// The exprMetadata type provides helper functions for retrieving source
// locations in a human readable manner based on the data contained within
// the expr.SourceInfo message.
type exprMetadata struct {
	info *exprpb.SourceInfo
}

func newExprMetadata(info *exprpb.SourceInfo) Metadata {
	return &exprMetadata{info: info}
}

func (m *exprMetadata) IDLocation(exprID int64) (common.Location, bool) {
	if exprOffset, found := m.IDOffset(exprID); found {
		var index = 0
		var lineIndex = 0
		var lineOffset int32
		for index, lineOffset = range m.info.LineOffsets {
			if lineOffset > exprOffset {
				break
			}
			lineIndex = index
		}
		line := lineIndex + 1
		column := exprOffset - lineOffset
		return common.NewLocation(line, int(column)), true
	}
	return nil, false
}

func (m *exprMetadata) IDOffset(exprID int64) (int32, bool) {
	position, found := m.info.Positions[exprID]
	return position, found
}
