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
	"fmt"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/test/proto3pb"
)

type indexedExpect struct {
	in  any
	out ref.Val
}

type indexedTestCase struct {
	name    string
	expr    string
	vars    []cel.EnvOption
	types   []any
	expects []indexedExpect
}

var (
	indexEvalTests = []indexedTestCase{
		{
			name: `ternary object graph`,
			expr: `!has(msg.child) ? 1 
				: has(msg.child.child) ? 2 
				: has(msg.child.payload.map_string_string.key) ? 3 
				: has(msg.child.payload) ? 4
				: 5`,
			vars: []cel.EnvOption{
				cel.Variable("msg", cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes")),
			},
			types: []any{&proto3pb.TestAllTypes{}},
			expects: []indexedExpect{
				{
					in: map[string]any{
						"msg": &proto3pb.NestedTestAllTypes{
							Payload: &proto3pb.TestAllTypes{},
						},
					},
					out: types.Int(1),
				},
				{
					in: map[string]any{
						"msg": &proto3pb.NestedTestAllTypes{
							Child: &proto3pb.NestedTestAllTypes{
								Child: &proto3pb.NestedTestAllTypes{},
							},
						},
					},
					out: types.Int(2),
				},
				{
					in: map[string]any{
						"msg": &proto3pb.NestedTestAllTypes{
							Child: &proto3pb.NestedTestAllTypes{
								Payload: &proto3pb.TestAllTypes{
									MapStringString: map[string]string{
										"key": "value",
									},
								},
							},
						},
					},
					out: types.Int(3),
				},
				{
					in: map[string]any{
						"msg": &proto3pb.NestedTestAllTypes{
							Child: &proto3pb.NestedTestAllTypes{
								Payload: &proto3pb.TestAllTypes{
									MapStringString: map[string]string{
										"wrong-key": "value",
									},
								},
							},
						},
					},
					out: types.Int(4),
				},
				{
					in: map[string]any{
						"msg": &proto3pb.NestedTestAllTypes{
							Child: &proto3pb.NestedTestAllTypes{
								Payload: &proto3pb.TestAllTypes{},
							},
						},
					},
					out: types.Int(4),
				},
				{
					in: map[string]any{
						"msg": &proto3pb.NestedTestAllTypes{
							Child: &proto3pb.NestedTestAllTypes{},
						},
					},
					out: types.Int(5),
				},
			},
		},
	}
)

func TestIndexedProgramEval(t *testing.T) {
	for _, tst := range indexEvalTests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			env := mustNewIndexedEnv(t, tc)
			prg := mustNewIndexedProgram(t, env, tc)
			for i, e := range tc.expects {
				ex := e
				t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
					got, _, err := prg.Eval(ex.in)
					if err != nil {
						t.Fatalf("prg.Eval(%v) failed: %v", ex.in, err)
					}
					if got.Equal(ex.out) != types.True {
						t.Errorf("prg.Eval(%v) got %v, wanted %v", ex.in, got, ex.out)
					}
				})
			}
		})
	}
}

func BenchmarkIndexedProgramEval(b *testing.B) {
	b.ResetTimer()
	for _, tst := range indexEvalTests {
		tc := tst
		b.Run(tc.name, func(b *testing.B) {
			env := mustNewIndexedEnv(b, tc)
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				b.Fatalf("env.Compile() failed: %v", iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				b.Fatalf("env.Program() failed: %v", err)
			}
			idxPrg := mustNewIndexedProgram(b, env, tc)
			for i, e := range tc.expects {
				ex := e
				b.Run(fmt.Sprintf("indexed/%d", i), func(b *testing.B) {
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						idxPrg.Eval(ex.in)
					}
				})
				b.Run(fmt.Sprintf("unindexed/%d", i), func(b *testing.B) {
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						prg.Eval(ex.in)
					}
				})
			}
		})
	}
}

func mustNewIndexedEnv(t testing.TB, tc indexedTestCase) *cel.Env {
	t.Helper()
	opts := []cel.EnvOption{cel.EnableMacroCallTracking()}
	opts = append(opts, tc.vars...)
	if len(tc.types) != 0 {
		opts = append(opts, cel.Types(tc.types...))
	}
	env, err := cel.NewEnv(opts...)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	return env
}

func mustNewIndexedProgram(t testing.TB, env *cel.Env, tc indexedTestCase) *IndexedProgram {
	ast, iss := env.Compile(tc.expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	idxr := NewIndexer()
	idxAST, err := idxr.GenerateIndex(env, ast)
	if err != nil {
		t.Fatalf("GenerateIndex() failed: %v", err)
	}
	prg, err := NewIndexedProgram(env, idxAST)
	if err != nil {
		t.Fatalf("NewIndexedProgram() failed: %v", err)
	}
	return prg
}
