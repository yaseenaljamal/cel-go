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
	"testing"

)

type testInfo struct {
	E string
	P string
}

var testCases = []testInfo{
	{
		E: `true && false`,
		P: `false`,
	},
	{
		E: `true && (false || x)`,
		P: `x`,
	},
	{
		E: `false && (false || x)`,
		P: `false`,
	},
	{
		E: `x && [1, 1u, 1.0].exists(y, type(y) == uint)`,
		P: `x`,
	},
	{
		E: `{"hello": "world".size(), "bytes":b"bytes-string"}`,
		P: `{"hello":5, "bytes":b"bytes-string"}`,
	},
	{
		E: `2 < 3`,
		P: `true`,
	},
	{
		E: `true ? x < 1.2 : y == ['hello']`,
		P: `_<_(x,1.2)`,
	},
}

func TestPrune(t *testing.T) {
	/*
	for i, tst := range testCases {
		pExpr := &exprpb.ParsedExpr{Expr: tst.E}
		state := NewEvalState()
		interpretable, _ := interpreter.NewUncheckedInterpretable(
			pExpr.Expr,
			ExhaustiveEval(state))
		interpretable.Eval(EmptyActivation())
		newExpr := PruneAst(pExpr.Expr, state)
		actual := debug.ToDebugString(newExpr)
		if !test.Compare(actual, tst.P) {
			t.Fatalf("prune[%d], diff: %s", i, test.DiffMessage("structure", actual, tst.P))
		}
	}*/
}
