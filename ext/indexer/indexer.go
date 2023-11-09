// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package indexer

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
)

const (
	maxFieldPatterns = 4
)

type indexer struct{}

// IndexedAST
type IndexedAST struct {
	// fields provides a list of presence fields sorted in descending frequency,
	// or if tied by ascending id.
	fields []*fieldFrequency

	// maskToASTSLot contains a set of possible valid bit masks corresponding to ASTs
	// where the mask assembled in reverse order to frequency. i.e. the highest frequency
	// field presence is encoded in the lowest bit, and the lowest frequency field presence
	// is encoded in the highest bit.
	maskToASTSlot map[uint8]int

	// asts contains a list of indexed ASTs which the code has attempted to prune down to
	// the minimal set by determining field presence dependencies.
	asts []*cel.Ast
}

func NewIndexer() *indexer {
	return &indexer{}
}

func (idxr *indexer) GenerateIndex(env *cel.Env, a *cel.Ast) (*IndexedAST, error) {
	folder, err := cel.NewConstantFoldingOptimizer()
	if err != nil {
		return nil, err
	}

	presenceFields := idxr.findFrequentPresenceFields(a.NativeRep())
	if len(presenceFields) == 0 {
		return &IndexedAST{
			fields:        []*fieldFrequency{},
			maskToASTSlot: map[uint8]int{0: 0},
			asts:          []*cel.Ast{a},
		}, nil
	}
	maskCount := 1 << len(presenceFields)
	indexedASTs := []*cel.Ast{}
	maskToASTSlot := make(map[uint8]int)
	for i := 0; i < maskCount; i++ {
		mask := uint8(i)
		effectiveMask := idxr.computeEffectiveMask(mask, presenceFields)
		if _, found := maskToASTSlot[effectiveMask]; found {
			continue
		}
		pr := newPresenceRewriter(mask, presenceFields)
		opt := cel.NewStaticOptimizer(pr, folder)
		indexed, iss := opt.Optimize(env, a)
		if iss.Err() != nil {
			return nil, iss.Err()
		}
		indexedASTs = append(indexedASTs, indexed)
		maskToASTSlot[mask] = len(indexedASTs) - 1
	}
	return &IndexedAST{
		fields:        presenceFields,
		maskToASTSlot: maskToASTSlot,
		asts:          indexedASTs,
	}, nil
}

func (idxr *indexer) findFrequentPresenceFields(a *ast.AST) []*fieldFrequency {
	root := ast.NavigateAST(a)
	presenceTests := ast.MatchDescendants(root, presenceTestMatcher)
	ft := newFieldTrie()
	for _, pt := range presenceTests {
		f := qualifiedFieldName(pt.AsSelect())
		ft.add(f, pt.ID())
	}
	// Pick the top N fields, meaning there are still 2^N possible index results.
	// In practice, the number of useful indices is much smaller, but for now we'll
	// start naive.
	frequentFields := ft.sortedPresenceFields()
	fieldCount := len(frequentFields)
	if fieldCount > maxFieldPatterns {
		fieldCount = maxFieldPatterns
	}
	return frequentFields[0:fieldCount]
}

func (idxr *indexer) computeEffectiveMask(mask uint8, presenceTests []*fieldFrequency) uint8 {
	effectiveMask := uint8(0)
	updates := make(map[int64]types.Bool, len(presenceTests))
	for i, pt := range presenceTests {
		bit := uint8(1 << i)
		updates[pt.id] = types.False
		// Since parent frequency is incremented during child presence tests, the parent
		// should always have a higher frequency than the child and thus be updated prior
		// to the child. This check skips impossible cases where a parent is absent, but
		// a child is present.
		if parentUpdate, found := updates[pt.parentID]; found && parentUpdate == types.False {
			continue
		}
		if mask&bit == bit {
			updates[pt.id] = types.True
			effectiveMask |= bit
		}
	}
	return effectiveMask
}

func newPresenceRewriter(mask uint8, presenceTests []*fieldFrequency) *presenceRewriter {
	updates := make(map[int64]types.Bool, len(presenceTests))
	for i, pt := range presenceTests {
		bit := uint8(1 << i)
		updates[pt.id] = types.False
		if mask&bit == bit {
			updates[pt.id] = types.True
		}
	}
	return &presenceRewriter{
		updates: updates,
	}
}

type presenceRewriter struct {
	updates map[int64]types.Bool
}

func (pr *presenceRewriter) Optimize(ctx *cel.OptimizerContext, a *ast.AST) *ast.AST {
	root := ast.NavigateAST(a)
	matches := ast.MatchDescendants(root, func(e ast.NavigableExpr) bool {
		_, found := pr.updates[e.ID()]
		return found
	})
	for _, match := range matches {
		match.SetKindCase(ctx.NewLiteral(pr.updates[match.ID()]))
	}
	return a
}

func presenceTestMatcher(e ast.NavigableExpr) bool {
	switch e.Kind() {
	case ast.SelectKind:
		sel := e.AsSelect()
		if !sel.IsTestOnly() {
			return false
		}
		return isFieldQualification(sel)
	}
	return false
}

func isFieldQualification(sel ast.SelectExpr) bool {
	op := sel.Operand()
	switch op.Kind() {
	case ast.IdentKind:
		return true
	case ast.SelectKind:
		return isFieldQualification(op.AsSelect())
	}
	return false
}

func qualifiedFieldName(sel ast.SelectExpr) string {
	op := sel.Operand()
	switch op.Kind() {
	case ast.IdentKind:
		return op.AsIdent() + "." + sel.FieldName()
	case ast.SelectKind:
		return qualifiedFieldName(op.AsSelect()) + "." + sel.FieldName()
	}
	panic("unreachable code encountered")
}
