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

package cel

import (
	"fmt"
	"log"
	"testing"

	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter/functions"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func Example() {
	// Create the CEL environment with declarations for the input attributes and
	// the desired extension functions. In many cases the desired functionality will
	// be present in a built-in function.
	decls := Declarations(
		// Identifiers used within this expression.
		decls.NewIdent("i", decls.String, nil),
		decls.NewIdent("you", decls.String, nil),
		// Function to generate a greeting from one person to another.
		//    i.greet(you)
		decls.NewFunction("greet",
			decls.NewInstanceOverload("greet_string_string",
				[]*exprpb.Type{decls.String, decls.String},
				decls.String)))
	e, err := NewEnv(decls)
	if err != nil {
		log.Fatalf("environment creation error: %s\n", err)
	}

	// Parse and check the expression.
	p, iss := e.Parse("i.greet(you)")
	if iss != nil && iss.Err() != nil {
		log.Fatalln(iss.Err())
	}
	c, iss := e.Check(p)
	if iss != nil && iss.Err() != nil {
		log.Fatalln(iss.Err())
	}

	// Create the program.
	funcs := Functions(
		&functions.Overload{
			Operator: "greet",
			Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
				return types.String(
					fmt.Sprintf("Hello %s! Nice to meet you, I'm %s.\n", rhs, lhs))
			}})
	prg, err := e.Program(c, funcs)
	if err != nil {
		log.Fatalf("program creation error: %s\n", err)
	}

	// Evaluate the program against some inputs.
	out, err := prg.Eval(Vars(map[string]interface{}{
		// Native values are converted to CEL values under the covers.
		"i": "CEL",
		// Values may also be lazily supplied.
		"you": func() ref.Val { return types.String("world") },
	}))
	if err != nil {
		log.Fatalf("runtime error: %s\n", err)
	}

	fmt.Println(out.Value())
	// Output:Hello world! Nice to meet you, I'm CEL.
}

func Test_ExampleWithBuiltins(t *testing.T) {
	// Variables used within this expression environment.
	decls := Declarations(
		decls.NewIdent("i", decls.String, nil),
		decls.NewIdent("you", decls.String, nil))
	env, err := NewEnv(decls)
	if err != nil {
		t.Fatalf("environment creation error: %s\n", err)
	}

	// Parse and type-check the expression.
	p, iss := env.Parse(`"Hello " + you + "! I'm " + i + "."`)
	if iss != nil && iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	c, iss := env.Check(p)
	if iss != nil && iss.Err() != nil {
		t.Fatal(iss.Err())
	}

	// Create the program, and evaluate it against some input.
	prg, err := env.Program(c)
	if err != nil {
		t.Fatalf("program creation error: %s\n", err)
	}
	out, err := prg.Eval(Vars(map[string]interface{}{
		"i":   "CEL",
		"you": "world"}))
	if err != nil {
		t.Fatalf("runtime error: %s\n", err)
	}

	// Hello world! I'm CEL.
	if out.Value().Equal(types.String("Hello world! I'm CEL.")) != types.True {
		t.Errorf(`Got '%v', wanted "Hello world! I'm CEL."`, out.Value())
	}
}

func Test_DisableStandardEnv(t *testing.T) {
	e, _ := NewEnv(
		ClearBuiltIns(),
		Declarations(decls.NewIdent("a.b.c", decls.Bool, nil)))

	t.Run("err", func(t *testing.T) {
		p, _ := e.Parse("a.b.c == true")
		_, iss := e.Check(p)
		if iss == nil || iss.Err() == nil {
			t.Error("Got successful check, expected error for missing operator '_==_'")
		}
	})

	t.Run("ok", func(t *testing.T) {
		p, _ := e.Parse("a.b.c")
		c, _ := e.Check(p)
		prg, _ := e.Program(c)
		out, _ := prg.Eval(Vars(map[string]interface{}{"a.b.c": true}))
		if !out.Value().Equal(types.True).(types.Bool) {
			t.Errorf("Got '%v', wanted 'true'", out.Value())
		}
	})
}

func Test_CustomTypes(t *testing.T) {
	e, _ := NewEnv(
		Package("google.api.expr.v1alpha1"),
		Types(&exprpb.Expr{}),
		Declarations(
			decls.NewIdent("expr",
				decls.NewObjectType("google.api.expr.v1alpha1.Expr"), nil)))

	p, _ := e.Parse(`
		expr == Expr{id: 2, 
			call_expr: Expr.Call{
				function: "_==_",
				args: [
					Expr{id: 1, ident_expr: Expr.Ident{ name: "a" }},
					Expr{id: 3, ident_expr: Expr.Ident{ name: "b" }}]
			}}`)
	c, _ := e.Check(p)
	prg, _ := e.Program(c)
	out, _ := prg.Eval(Vars(map[string]interface{}{"expr": &exprpb.Expr{
		Id: 2,
		ExprKind: &exprpb.Expr_CallExpr{
			CallExpr: &exprpb.Expr_Call{
				Function: "_==_",
				Args: []*exprpb.Expr{
					&exprpb.Expr{
						Id: 1,
						ExprKind: &exprpb.Expr_IdentExpr{
							IdentExpr: &exprpb.Expr_Ident{Name: "a"},
						},
					},
					&exprpb.Expr{
						Id: 3,
						ExprKind: &exprpb.Expr_IdentExpr{
							IdentExpr: &exprpb.Expr_Ident{Name: "b"},
						},
					},
				},
			},
		},
	}}))
	if !out.Value().Equal(types.True).(types.Bool) {
		t.Errorf("Got '%v', wanted 'true'", out.Value())
	}
}

func Test_GlobalVars(t *testing.T) {
	mapStrDyn := decls.NewMapType(decls.String, decls.Dyn)
	e, _ := NewEnv(
		Declarations(
			decls.NewIdent("attrs", mapStrDyn, nil),
			decls.NewIdent("default", decls.Dyn, nil),
			decls.NewFunction(
				"get",
				decls.NewInstanceOverload(
					"get_map",
					[]*exprpb.Type{mapStrDyn, decls.String, decls.Dyn},
					decls.Dyn))))
	p, _ := e.Parse(`attrs.get("first", attrs.get("second", default))`)
	c, _ := e.Check(p)

	// Create the program.
	funcs := Functions(
		&functions.Overload{
			Operator: "get",
			Function: func(args ...ref.Val) ref.Val {
				if len(args) != 3 {
					return types.NewErr("invalid arguments to 'get'")
				}
				attrs, ok := args[0].(traits.Mapper)
				if !ok {
					return types.NewErr(
						"invalid operand of type '%v' to obj.get(key, def)",
						args[0].Type())
				}
				key, ok := args[1].(types.String)
				if !ok {
					return types.NewErr(
						"invalid key of type '%v' to obj.get(key, def)",
						args[1].Type())
				}
				defVal := args[2]
				if attrs.Contains(key) == types.True {
					return attrs.Get(key)
				}
				return defVal
			}})
	prg, _ := e.Program(c, funcs, Globals(Vars(map[string]interface{}{
		"default": "third",
	})))

	t.Run("global_default", func(t *testing.T) {
		out, _ := prg.Eval(Vars(map[string]interface{}{
			"attrs": map[string]interface{}{}}))
		if out.Value().Equal(types.String("third")) != types.True {
			t.Errorf("Got '%v', expected 'third'.", out.Value())
		}
	})

	t.Run("attrs_alt", func(t *testing.T) {
		out, _ := prg.Eval(Vars(map[string]interface{}{
			"attrs": map[string]interface{}{"second": "yep"}}))
		if out.Value().Equal(types.String("yep")) != types.True {
			t.Errorf("Got '%v', expected 'yep'.", out.Value())
		}
	})

	t.Run("local_default", func(t *testing.T) {
		out, _ := prg.Eval(Vars(map[string]interface{}{
			"attrs":   map[string]interface{}{},
			"default": "fourth"}))
		if out.Value().Equal(types.String("fourth")) != types.True {
			t.Errorf("Got '%v', expected 'fourth'.", out.Value())
		}
	})
}

func Test_EvalOptions(t *testing.T) {
	e, _ := NewEnv(
		Declarations(
			decls.NewIdent("k", decls.String, nil),
			decls.NewIdent("v", decls.Bool, nil)))
	p, _ := e.Parse(`{k: true}[k] || v != false`)
	c, _ := e.Check(p)

	prg, err := e.Program(c, EvalOptions(OptExhaustiveEval))
	if err != nil {
		t.Fatalf("program creation error: %s\n", err)
	}
	out, err := prg.Eval(Vars(map[string]interface{}{"k": "key", "v": true}))
	if err != nil {
		t.Fatalf("runtime error: %s\n", err)
	}
	if out.Value().Equal(types.True) != types.True {
		t.Errorf("Got '%v', expected 'true'", out.Value())
	}

	// Test to see whether 'v != false' was resolved to a value.
	// With short-circuiting it normally wouldn't be.
	s := out.State()
	rhsVal, found := s.Value(p.Value().GetCallExpr().GetArgs()[1].Id)
	if !found {
		t.Error("Got not found, wanted evaluation of right hand side expression.")
		return
	}
	if rhsVal.Equal(types.True) != types.True {
		t.Errorf("Got '%v', expected 'true'", rhsVal)
	}
}
