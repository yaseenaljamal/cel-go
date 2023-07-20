// Copyright 2023 Google LLC
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

// Package ast declares data structures useful for parsed and checked abstract syntax trees
package ast

import (
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type AST struct {
	expr         Expr
	SourceInfo   *SourceInfo
	TypeMap      map[int64]*types.Type
	ReferenceMap map[int64]*ReferenceInfo
	checked      bool
}

func (a *AST) Expr() Expr {
	return a.expr
}

func (a *AST) GetType(id int64) *types.Type {
	if t, found := a.TypeMap[id]; found {
		return t
	}
	return types.DynType
}

func (a *AST) GetOverloadIDs(id int64) []string {
	if ref, found := a.ReferenceMap[id]; found {
		return ref.OverloadIDs
	}
	return []string{}
}

func (a *AST) IsChecked() bool {
	return a.checked
}

func NewAST(e Expr, sourceInfo *SourceInfo) *AST {
	return &AST{
		expr:         e,
		SourceInfo:   sourceInfo,
		TypeMap:      make(map[int64]*types.Type),
		ReferenceMap: make(map[int64]*ReferenceInfo),
		checked:      false,
	}
}

func NewCheckedAST(in *AST, typeMap map[int64]*types.Type, refMap map[int64]*ReferenceInfo) *AST {
	return &AST{
		expr:         in.expr,
		SourceInfo:   in.SourceInfo,
		TypeMap:      typeMap,
		ReferenceMap: refMap,
		checked:      true,
	}
}

func NewSourceInfo(src common.Source) *SourceInfo {
	var lineOffsets []int32
	var desc string
	if src != nil {
		desc = src.Description()
		lineOffsets = src.LineOffsets()
	}
	return &SourceInfo{
		desc:         desc,
		lines:        lineOffsets,
		offsetRanges: make(map[int64]OffsetRange),
		macroCalls:   make(map[int64]Expr),
	}
}

type SourceInfo struct {
	syntax       string
	desc         string
	lines        []int32
	offsetRanges map[int64]OffsetRange
	macroCalls   map[int64]Expr
}

func (s *SourceInfo) LineOffsets() []int32 {
	if s == nil {
		return []int32{}
	}
	return s.lines
}

func (s *SourceInfo) MacroCalls() map[int64]Expr {
	if s == nil {
		return make(map[int64]Expr, 0)
	}
	return s.macroCalls
}

func (s *SourceInfo) GetMacroCall(id int64) (Expr, bool) {
	if s == nil {
		return nil, false
	}
	e, found := s.macroCalls[id]
	return e, found
}

func (s *SourceInfo) SetMacroCall(id int64, e Expr) {
	if s == nil {
		return
	}
	s.macroCalls[id] = e
}

func (s *SourceInfo) GetOffsetRange(id int64) (OffsetRange, bool) {
	if s == nil {
		return OffsetRange{}, false
	}
	o, found := s.offsetRanges[id]
	return o, found
}

func (s *SourceInfo) SetOffsetRange(id int64, o OffsetRange) {
	if s == nil {
		return
	}
	s.offsetRanges[id] = o
}

func (s *SourceInfo) GetStartLocation(id int64) common.Location {
	var line = 1
	if o, found := s.GetOffsetRange(id); found {
		col := int(o.Start)
		for _, lineOffset := range s.LineOffsets() {
			if lineOffset < o.Start {
				line++
				col = int(o.Start - lineOffset)
			} else {
				break
			}
		}
		return common.NewLocation(line, col)
	}
	return common.NoLocation
}

func (s *SourceInfo) GetStopLocation(id int64) common.Location {
	var line = 1
	if o, found := s.GetOffsetRange(id); found {
		col := int(o.Stop)
		for _, lineOffset := range s.LineOffsets() {
			if lineOffset < o.Stop {
				line++
				col = int(o.Stop - lineOffset)
			} else {
				break
			}
		}
		return common.NewLocation(line, col)
	}
	return common.NoLocation
}

func (s *SourceInfo) ComputeOffset(line, col int32) int32 {
	if line == 1 {
		return col
	}
	if line < 1 || line > int32(len(s.lines)) {
		return -1
	}
	offset := s.LineOffsets()[line-2]
	return offset + col
}

type OffsetRange struct {
	Start int32
	Stop  int32
}

// ReferenceInfo contains a CEL native representation of an identifier reference which may refer to
// either a qualified identifier name, a set of overload ids, or a constant value from an enum.
type ReferenceInfo struct {
	Name        string
	OverloadIDs []string
	Value       ref.Val
}

// NewIdentReference creates a ReferenceInfo instance for an identifier with an optional constant value.
func NewIdentReference(name string, value ref.Val) *ReferenceInfo {
	return &ReferenceInfo{Name: name, Value: value}
}

// NewFunctionReference creates a ReferenceInfo instance for a set of function overloads.
func NewFunctionReference(overloads ...string) *ReferenceInfo {
	info := &ReferenceInfo{}
	for _, id := range overloads {
		info.AddOverload(id)
	}
	return info
}

// AddOverload appends a function overload ID to the ReferenceInfo.
func (r *ReferenceInfo) AddOverload(overloadID string) {
	for _, id := range r.OverloadIDs {
		if id == overloadID {
			return
		}
	}
	r.OverloadIDs = append(r.OverloadIDs, overloadID)
}

// Equals returns whether two references are identical to each other.
func (r *ReferenceInfo) Equals(other *ReferenceInfo) bool {
	if r.Name != other.Name {
		return false
	}
	if len(r.OverloadIDs) != len(other.OverloadIDs) {
		return false
	}
	if len(r.OverloadIDs) != 0 {
		overloadMap := make(map[string]struct{}, len(r.OverloadIDs))
		for _, id := range r.OverloadIDs {
			overloadMap[id] = struct{}{}
		}
		for _, id := range other.OverloadIDs {
			_, found := overloadMap[id]
			if !found {
				return false
			}
		}
	}
	if r.Value == nil && other.Value == nil {
		return true
	}
	if r.Value == nil && other.Value != nil ||
		r.Value != nil && other.Value == nil ||
		r.Value.Equal(other.Value) != types.True {
		return false
	}
	return true
}
