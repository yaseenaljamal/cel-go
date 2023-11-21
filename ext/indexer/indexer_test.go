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
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
)

func TestGenerateIndex(t *testing.T) {
	tests := []struct {
		expr       string
		vars       []cel.EnvOption
		types      []any
		idxFields  map[string]int
		maskToSlot map[uint8]int
		idxASTs    []string
	}{
		{
			expr: `has(a.b) ? a : b`,
			vars: []cel.EnvOption{
				cel.Variable("a", cel.MapType(cel.StringType, cel.StringType)),
				cel.Variable("b", cel.MapType(cel.StringType, cel.StringType)),
			},
			types:      []any{},
			idxFields:  map[string]int{"a.b": 1},
			maskToSlot: map[uint8]int{0: 0, 1: 1},
			idxASTs:    []string{`b`, `a`},
		},
		{
			expr: `has(a.b) ? a : has(b.c) ? b : c`,
			vars: []cel.EnvOption{
				cel.Variable("a", cel.MapType(cel.StringType, cel.StringType)),
				cel.Variable("b", cel.MapType(cel.StringType, cel.StringType)),
				cel.Variable("c", cel.MapType(cel.StringType, cel.StringType)),
			},
			types:      []any{},
			idxFields:  map[string]int{"a.b": 1, "b.c": 1},
			maskToSlot: map[uint8]int{0: 0, 1: 1, 2: 2, 3: 3},
			idxASTs:    []string{`c`, `a`, `b`, `a`},
		},
		{
			expr: `!has(a.b) ? a.c : has(b.c) ? b.c : c.d`,
			vars: []cel.EnvOption{
				cel.Variable("a", cel.MapType(cel.StringType, cel.StringType)),
				cel.Variable("b", cel.MapType(cel.StringType, cel.StringType)),
				cel.Variable("c", cel.MapType(cel.StringType, cel.StringType)),
			},
			types:      []any{},
			idxFields:  map[string]int{"a.b": 1, "b.c": 1},
			maskToSlot: map[uint8]int{0: 0, 1: 1, 2: 2, 3: 3},
			idxASTs:    []string{`a.c`, `c.d`, `a.c`, `b.c`},
		},
		{
			expr: `has(a.b) && has(a.b.c) ? a.b.c : !has(b.c) ? b.c : c.d`,
			vars: []cel.EnvOption{
				cel.Variable("a", cel.MapType(cel.StringType, cel.MapType(cel.StringType, cel.StringType))),
				cel.Variable("b", cel.MapType(cel.StringType, cel.StringType)),
				cel.Variable("c", cel.MapType(cel.StringType, cel.StringType)),
			},
			types:     []any{},
			idxFields: map[string]int{"a.b": 2, "a.b.c": 1, "b.c": 1},
			// note, slots 2 and 6 are dropped out since ...
			// 2 (010) implies a.b is not present, but a.b.c is present
			// 6 (110) implies the same as 4 (100) which has the same implication as 2
			maskToSlot: map[uint8]int{
				0: 0,
				1: 1,
				2: 0,
				3: 2,
				4: 3,
				5: 4,
				6: 3,
				7: 5,
			},
			idxASTs: []string{`b.c`, `b.c`, `a.b.c`, `c.d`, `c.d`, `a.b.c`},
		},
	}

	idxr := NewIndexer()
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			opts := []cel.EnvOption{cel.EnableMacroCallTracking()}
			opts = append(opts, tc.vars...)
			if len(tc.types) != 0 {
				opts = append(opts, cel.Types(tc.types...))
			}
			env, err := cel.NewEnv(opts...)
			if err != nil {
				t.Fatalf("cel.NewEnv() failed: %v", err)
			}
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile() failed: %v", iss.Err())
			}
			idx, err := idxr.GenerateIndex(env, ast)
			if err != nil {
				t.Fatalf("GenerateIndex() failed: %v", err)
			}
			idxFields := make(map[string]int, len(idx.Fields))
			for _, f := range idx.Fields {
				idxFields[f.field] = f.frequency
			}
			idxASTs := make([]string, 0, len(idx.ASTs))
			for _, a := range idx.ASTs {
				strAST, err := cel.AstToString(a)
				if err != nil {
					t.Fatalf("cel.AstToString(%v) failed: %v", a, err)
				}
				idxASTs = append(idxASTs, strAST)
			}
			if !reflect.DeepEqual(tc.idxFields, idxFields) {
				t.Errorf("index fields got %v, wanted %v", idxFields, tc.idxFields)
			}
			if !reflect.DeepEqual(tc.idxASTs, idxASTs) {
				t.Errorf("index asts got %v, wanted %v", idxASTs, tc.idxASTs)
			}
			if !reflect.DeepEqual(tc.maskToSlot, idx.MaskToASTSlot) {
				t.Errorf("index mask to ast slot got %v, wanted %v", idx.MaskToASTSlot, tc.maskToSlot)
			}
		})
	}
}
