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
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/packages"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter/functions"
	"github.com/google/cel-go/parser"
	"github.com/google/cel-go/test"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

type testCase struct {
	name string
	E    string
	I    map[string]interface{}
}

func TestExhaustiveInterpreter_ConditionalExpr(t *testing.T) {
	// a ? b < 1.0 : c == ["hello"]
	// Operator "_==_" is at Expr 6, should be evaluated in exhaustive mode
	// even though "a" is true
	program, err := NewProgram(
		test.Conditional.Expr,
		test.Conditional.Info(t.Name()),
		ExhaustiveProgram(true))
	if err != nil {
		t.Fatal(err)
	}
	interpretable := interpreter.NewInterpretable(program)
	result, state := interpretable.Eval(
		NewActivation(map[string]interface{}{
			"a": true,
			"b": 0.999,
			"c": types.NewStringList([]string{"hello"})}))
	ev, _ := state.Value(6)
	// "==" should be evaluated in exhaustive mode though unnecessary
	if ev != types.True {
		t.Errorf("Else expression expected to be true, got: %v", ev)
	}
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestExhaustiveInterpreter_ConditionalExprErr(t *testing.T) {
	// a ? b < 1.0 : c == ["hello"]
	// Operator "<" is at Expr 3, "_==_" is at Expr 6.
	// Both should be evaluated in exhaustive mode though a is not provided
	program, err := NewProgram(
		test.Conditional.Expr,
		test.Conditional.Info(t.Name()),
		ExhaustiveProgram(true))
	if err != nil {
		t.Fatal(err)
	}

	interpretable := interpreter.NewInterpretable(program)
	result, state := interpretable.Eval(
		NewActivation(map[string]interface{}{
			"b": 1.001,
			"c": types.NewStringList([]string{"hello"})}))
	iv, _ := state.Value(3)
	// "<" should be evaluated in exhaustive mode though unnecessary
	if iv != types.False {
		t.Errorf("If expression expected to be false, got: %v", iv)
	}
	ev, _ := state.Value(6)
	// "==" should be evaluated in exhaustive mode though unnecessary
	if ev != types.True {
		t.Errorf("Else expression expected to be true, got: %v", ev)
	}
	if result.Type() != types.UnknownType {
		t.Errorf("Expected unknown result, got: %v", result)
	}
}

func TestExhaustiveInterpreter_LogicalOrEquals(t *testing.T) {
	// a || b == "b"
	// Operator "==" is at Expr 4, should be evaluated though "a" is true
	program, err := NewProgram(
		test.LogicalOrEquals.Expr,
		test.LogicalOrEquals.Info(t.Name()),
		ExhaustiveProgram(true))
	if err != nil {
		t.Fatal(err)
	}

	// TODO: make the type identifiers part of the standard declaration set.
	provider := types.NewProvider(&exprpb.Expr{})
	i := NewStandardInterpreter(packages.NewPackage("test"), provider)
	interpretable := i.NewInterpretable(program)
	result, state := interpretable.Eval(
		NewActivation(map[string]interface{}{
			"a": true,
			"b": "b",
		}))
	rhv, _ := state.Value(4)
	// "==" should be evaluated in exhaustive mode though unnecessary
	if rhv != types.True {
		t.Errorf("Right hand side expression expected to be true, got: %v", rhv)
	}
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestInterpreter_CallExpr(t *testing.T) {
	program, err := NewProgram(
		test.Equality.Expr,
		test.Equality.Info(t.Name()),
		ExhaustiveProgram(true))
	if err != nil {
		t.Fatal(err)
	}
	intr := NewStandardInterpreter(
		packages.NewPackage("google.api.expr"),
		types.NewProvider(&exprpb.ParsedExpr{}))
	interpretable := intr.NewInterpretable(program)
	result, state := interpretable.Eval(
		NewActivation(map[string]interface{}{"a": int64(41)}))
	if result != types.False {
		t.Errorf("Expected false, got: %v", result)
	}
	if ident, found := state.Value(1); !found || ident != types.Int(41) {
		t.Errorf("State of ident 'a' != 41, got: %v", ident)
	}
}

func TestInterpreter_SelectExpr(t *testing.T) {
	program, _ := NewProgram(
		test.Select.Expr,
		test.Select.Info(t.Name()))

	interpretable := interpreter.NewInterpretable(program)
	result, _ := interpretable.Eval(
		NewActivation(map[string]interface{}{
			"a.b": types.NewDynamicMap(map[string]bool{"c": true}),
		}))
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestInterpreter_ConditionalExpr(t *testing.T) {
	// a ? b < 1.0 : c == ["hello"]
	// Operator "<" is at Expr 3, "_==_" is at Expr 6.
	program, _ := NewProgram(
		test.Conditional.Expr,
		test.Conditional.Info(t.Name()))

	interpretable := interpreter.NewInterpretable(program)
	result, _ := interpretable.Eval(
		NewActivation(map[string]interface{}{
			"a": true,
			"b": 0.999,
			"c": types.NewStringList([]string{"hello"})}))
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestInterpreter_ComprehensionExpr(t *testing.T) {
	result, _ := evalExpr(t, "[1, 1u, 1.0].exists(x, type(x) == uint)")
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_NonStrictExistsComprehension(t *testing.T) {
	result, _ := evalExpr(t, "[0, 2, 4].exists(x, 4/x == 2 && 4/(4-x) == 2)")
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_NonStrictAllComprehension(t *testing.T) {
	result, _ := evalExpr(t, "![0, 2, 4].all(x, 4/x != 2 && 4/(4-x) != 2)")
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_NonStrictAllWithInput(t *testing.T) {
	parsed := parseExpr(t,
		`code == "111" && ["a", "b"].all(x, x in tags)
		|| code == "222" && ["a", "b"].all(x, x in tags)`)
	pgrm, _ := NewProgram(parsed.Expr, parsed.SourceInfo)
	i := interpreter.NewInterpretable(pgrm)
	result, _ := i.Eval(NewActivation(map[string]interface{}{
		"code": "222",
		"tags": []string{"a", "b"},
	}))
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_LongQualifiedIdent(t *testing.T) {
	parsed := parseExpr(t, `a.b.c.d == 10`)
	pgrm, _ := NewProgram(parsed.Expr, parsed.SourceInfo)
	i := interpreter.NewInterpretable(pgrm)
	result, _ := i.Eval(NewActivation(map[string]interface{}{
		"a.b.c.d": 10,
	}))
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_FieldAccess(t *testing.T) {
	parsed := parseExpr(t, `val.input.expr.id == 10`)
	pgrm, _ := NewProgram(parsed.Expr, parsed.SourceInfo)
	i := interpreter.NewInterpretable(pgrm)
	unk, _ := i.Eval(NewActivation(map[string]interface{}{}))
	if !types.IsUnknown(unk) {
		t.Errorf("Got %v, wanted unknown", unk)
	}
	result, _ := i.Eval(NewActivation(map[string]interface{}{
		"val.input": &exprpb.ParsedExpr{Expr: &exprpb.Expr{Id: 10}},
	}))
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_ExistsOne(t *testing.T) {
	result, _ := evalExpr(t, "[1, 2, 3].exists_one(x, (x % 2) == 0)")
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_Map(t *testing.T) {
	result, _ := evalExpr(t, "[1, 2, 3].map(x, x * 2) == [2, 4, 6]")
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_Filter(t *testing.T) {
	result, _ := evalExpr(t, "[1, 2, 3].filter(x, x > 2) == [3]")
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_Timestamp(t *testing.T) {
	result, _ := evalExpr(t, "timestamp('2001-01-01T01:23:45Z').getDayOfWeek() == 1")
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_HasTest(t *testing.T) {
	result, _ := evalExpr(t,
		`has({'a':1}.a) &&
		 !has({}.a) &&
		 has(google.api.expr.v1alpha1.ParsedExpr{
			expr:google.api.expr.v1alpha1.Expr{id: 1}}
			.expr) &&
		 !has(google.api.expr.v1alpha1.ParsedExpr{
			expr:google.api.expr.v1alpha1.Expr{id: 1}}
			.source_info)`)
	if result != types.True {
		t.Errorf("Got %v, wanted true", result)
	}
}

func TestInterpreter_LogicalAnd(t *testing.T) {
	// a && {c: true}.c
	program, _ := NewProgram(
		test.LogicalAnd.Expr,
		test.LogicalAnd.Info(t.Name()))

	interpretable := interpreter.NewInterpretable(program)
	// TODO: make the type identifiers part of the standard declaration set.
	result, _ := interpretable.Eval(
		NewActivation(map[string]interface{}{"a": true}))
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestInterpreter_LogicalAndMissingType(t *testing.T) {
	// a && {c: true}.c
	program, _ := NewProgram(
		test.LogicalAndMissingType.Expr,
		test.LogicalAndMissingType.Info(t.Name()))

	interpretable := interpreter.NewInterpretable(program)
	result, _ := interpretable.Eval(
		NewActivation(map[string]interface{}{"a": false}))
	if result != types.False {
		t.Errorf("Got: %v, wanted true", result)
	}
	result, _ = interpretable.Eval(
		NewActivation(map[string]interface{}{"a": true}))
	if !types.IsError(result) {
		t.Errorf("Got: %v, wanted error", result)
	}
}

func TestInterpreter_LogicalOr(t *testing.T) {
	// {c: false}.c || a
	program, _ := NewProgram(
		test.LogicalOr.Expr,
		test.LogicalOr.Info(t.Name()))

	// TODO: make the type identifiers part of the standard declaration set.
	provider := types.NewProvider(&exprpb.Expr{})
	i := NewStandardInterpreter(packages.NewPackage("test"), provider)
	interpretable := i.NewInterpretable(program)
	result, _ := interpretable.Eval(
		NewActivation(map[string]interface{}{"a": true}))
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestInterpreter_LogicalOrEquals(t *testing.T) {
	// a || b == "b"
	// Operator "==" is at Expr 4, should not be evaluated since "a" is true
	program, _ := NewProgram(
		test.LogicalOrEquals.Expr,
		test.LogicalOrEquals.Info(t.Name()))

	// TODO: make the type identifiers part of the standard declaration set.
	provider := types.NewProvider(&exprpb.Expr{})
	i := NewStandardInterpreter(packages.NewPackage("test"), provider)
	interpretable := i.NewInterpretable(program)
	result, _ := interpretable.Eval(
		NewActivation(map[string]interface{}{
			"a": true,
			"b": "b",
		}))
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestInterpreter_BuildObject(t *testing.T) {
	src := common.NewTextSource(
		"v1alpha1.Expr{id: 1, " +
			"const_expr: v1alpha1.Constant{ " +
			"string_value: \"oneof_test\"}}")
	parsed, errors := parser.Parse(src)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	pkgr := packages.NewPackage("google.api.expr")
	provider := types.NewProvider(&exprpb.Expr{})
	env := checker.NewStandardEnv(pkgr, provider)
	checked, errors := checker.Check(parsed, src, env)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	i := NewStandardInterpreter(pkgr, provider)
	prog, _ := NewCheckedProgram(checked)
	eval := i.NewInterpretable(prog)
	result, _ := eval.Eval(emptyActivation)
	expected := &exprpb.Expr{Id: 1,
		ExprKind: &exprpb.Expr_ConstExpr{
			ConstExpr: &exprpb.Constant{
				ConstantKind: &exprpb.Constant_StringValue{
					StringValue: "oneof_test"}}}}
	if !proto.Equal(result.(ref.Value).Value().(proto.Message), expected) {
		t.Errorf("Could not build object properly. Got '%v', wanted '%v'",
			result.(ref.Value).Value(),
			expected)
	}
}

func TestInterpreter_GetObjectEnumField(t *testing.T) {
	src := common.NewTextSource("a.repeated_nested_enum[0]")
	parsed, errors := parser.Parse(src)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	pkgr := packages.NewPackage("google.api.tools.expr.test")
	provider := types.NewProvider(&test.TestAllTypes{})
	env := checker.NewStandardEnv(pkgr, provider)
	env.Add(decls.NewIdent("a", decls.NewObjectType("google.api.tools.expr.test.TestAllTypes"), nil))
	checked, errors := checker.Check(parsed, src, env)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	i := NewStandardInterpreter(pkgr, provider)
	prog, _ := NewCheckedProgram(checked)
	eval := i.NewInterpretable(prog)
	a := &test.TestAllTypes{
		RepeatedNestedEnum: []test.TestAllTypes_NestedEnum{
			test.TestAllTypes_BAR,
		},
	}
	result, state := eval.Eval(NewActivation(map[string]interface{}{
		"a": types.NewObject(a),
	}))
	expected := int64(1)
	got, ok := result.(ref.Value).Value().(int64)
	if !ok {
		t.Fatalf("cannot cast result to int64: result=%v state=%v", result, state)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Could not build object properly. Got '%v', wanted '%v'",
			result.(ref.Value).Value(),
			expected)
	}
}

func TestInterpreter_SetObjectEnumField(t *testing.T) {
	// Test the use of enums within object construction, and their equivalence
	// int values within CEL.
	src := common.NewTextSource(
		`TestAllTypes{
			repeated_nested_enum: [
				0,
				TestAllTypes.NestedEnum.BAZ,
				TestAllTypes.NestedEnum.BAR],
			repeated_int32: [
				TestAllTypes.NestedEnum.FOO,
				TestAllTypes.NestedEnum.BAZ]}`)
	parsed, errors := parser.Parse(src)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	pkgr := packages.NewPackage("google.api.tools.expr.test")
	provider := types.NewProvider(&test.TestAllTypes{})
	env := checker.NewStandardEnv(pkgr, provider)
	checked, errors := checker.Check(parsed, src, env)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	i := NewStandardInterpreter(pkgr, provider)
	prog, _ := NewCheckedProgram(checked)
	eval := i.NewInterpretable(prog)
	expected := &test.TestAllTypes{
		RepeatedNestedEnum: []test.TestAllTypes_NestedEnum{
			test.TestAllTypes_FOO,
			test.TestAllTypes_BAZ,
			test.TestAllTypes_BAR,
		},
		RepeatedInt32: []int32{
			int32(0),
			int32(2),
		},
	}
	result, state := eval.Eval(NewActivation(map[string]interface{}{}))
	got, ok := result.(ref.Value).Value().(*test.TestAllTypes)
	if !ok {
		t.Fatalf("cannot cast result to int64: result=%v state=%v", result, state)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Could not build object properly. Got '%v', wanted '%v'",
			result.(ref.Value).Value(),
			expected)
	}
}

func TestInterpreter_ConstantReturnValue(t *testing.T) {
	parsed, err := parser.Parse(common.NewTextSource("42"))
	if len(err.GetErrors()) != 0 {
		t.Error(err)
	}
	prg, _ := NewProgram(parsed.GetExpr(), parsed.GetSourceInfo())
	i := interpreter.NewInterpretable(prg)
	res, _ := i.Eval(emptyActivation)
	if int64(res.(types.Int)) != int64(42) {
		t.Errorf("Got '%v', wanted 1", res)
	}
}

func TestInterpreter_InList(t *testing.T) {
	parsed, err := parser.Parse(common.NewTextSource("1 in [1, 2, 3]"))
	if len(err.GetErrors()) != 0 {
		t.Error(err)
	}
	prg, _ := NewProgram(parsed.GetExpr(), parsed.GetSourceInfo())
	i := interpreter.NewInterpretable(prg)
	res, _ := i.Eval(emptyActivation)
	if res != types.True {
		t.Errorf("Got '%v', wanted 'true'", res)
	}
}

func TestInterpreter_BuildMap(t *testing.T) {
	parsed, err := parser.Parse(common.NewTextSource("{'b': '''hi''', 'c': name}"))
	if len(err.GetErrors()) != 0 {
		t.Error(err)
	}
	prg, _ := NewProgram(parsed.GetExpr(), parsed.GetSourceInfo())
	i := interpreter.NewInterpretable(prg)
	res, _ := i.Eval(NewActivation(map[string]interface{}{"name": "tristan"}))
	value, _ := res.(ref.Value).ConvertToNative(
		reflect.TypeOf(map[string]string{}))
	mapVal := value.(map[string]string)
	if mapVal["b"] != "hi" || mapVal["c"] != "tristan" {
		t.Errorf("Got '%v', expected map[b:hi c:tristan]", value)
	}
}

func TestInterpreter_MapIndex(t *testing.T) {
	parsed, err := parser.Parse(common.NewTextSource("{'a':1}['a']"))
	if len(err.GetErrors()) != 0 {
		t.Error(err)
	}
	prg, _ := NewProgram(parsed.GetExpr(), parsed.GetSourceInfo())
	i := interpreter.NewInterpretable(prg)
	res, _ := i.Eval(emptyActivation)
	if res != types.Int(1) {
		t.Errorf("Got '%v', wanted 1", res)
	}
}

func TestInterpreter_Matches(t *testing.T) {
	expression := "input.matches('k.*')"
	expr := compileExpr(t, expression, decls.NewIdent("input", decls.String, nil))
	prog, _ := NewCheckedProgram(expr)
	eval := interpreter.NewInterpretable(prog)

	for input, expectedResult := range map[string]bool{
		"kathmandu":   true,
		"foo":         false,
		"bar":         false,
		"kilimanjaro": true,
	} {
		result, _ := eval.Eval(NewActivation(map[string]interface{}{
			"input": input,
		}))
		if v, ok := result.Value().(bool); !ok || v != expectedResult {
			t.Errorf("Got %v, wanted %v for expr %s with input %s", result.Value(), expectedResult, expression, input)
		}
	}
}

func BenchmarkInterpreter_ConditionalExpr(b *testing.B) {
	// a ? b < 1.0 : c == ["hello"]
	program, _ := NewProgram(
		test.Conditional.Expr,
		test.Conditional.Info(b.Name()),
		OptimizeProgram(true))
	interpretable := interpreter.NewInterpretable(program)
	activation := NewActivation(map[string]interface{}{
		"a": types.False,
		"b": types.Double(0.999),
		"c": types.NativeToValue([]string{"hello"})})
	for i := 0; i < b.N; i++ {
		interpretable.Eval(activation)
	}
}

func BenchmarkInterpreter_EqualsCall(b *testing.B) {
	// type(a) == uint
	activation := NewActivation(map[string]interface{}{
		"a": types.Uint(20)})
	d := NewDispatcher()
	d.Add(functions.StandardOverloads()...)
	evalState := newEvalState(4)
	for i := 0; i < b.N; i++ {
		xRef, _ := activation.ResolveName("a")
		evalState.values[1] = xRef
		xRef, _ = evalState.Value(1)
		typeOfXRef := xRef.ConvertToType(types.TypeType)
		evalState.values[2] = typeOfXRef
		typeOfXRef, _ = evalState.Value(2)
		evalState.values[3] = typeOfXRef.Equal(types.UintType)
	}
}

func BenchmarkInterpreter_EqualsDispatch(b *testing.B) {
	// type(a) == uint
	activation := NewActivation(map[string]interface{}{
		"a": types.Uint(20)})
	evalState := newEvalState(4)
	d := NewDispatcher()
	d.Add(functions.StandardOverloads()...)
	p := types.NewProvider()
	uintType, _ := p.FindIdent("uint")
	callTypeOf := NewCall(2, "type", []int64{1})
	callEq := NewCall(3, "_==_", []int64{1, 2})
	for i := 0; i < b.N; i++ {
		xRef, _ := activation.ResolveName("a")
		evalState.values[1] = xRef
		d.Dispatch(evalState.values, callTypeOf)
		// not-found here.
		activation.ResolveName("uint")
		evalState.values[3] = uintType
		d.Dispatch(evalState.values, callEq)
	}
}

func BenchmarkInterpreter_EqualInstructions(b *testing.B) {
	// type(a) == uint
	program, _ := NewProgram(
		test.TypeEquality.Expr,
		test.TypeEquality.Info(b.Name()),
		OptimizeProgram(true))
	interpretable := interpreter.NewInterpretable(program)
	activation := NewActivation(map[string]interface{}{
		"a": types.Uint(20)})
	for i := 0; i < b.N; i++ {
		interpretable.Eval(activation)
	}
}

func BenchmarkInterpreter_ComprehensionExpr(b *testing.B) {
	// [1, 1u, 1.0].exists(x, type(x) == uint)
	program, _ := NewProgram(
		test.Exists.Expr,
		test.Exists.Info(b.Name()),
		OptimizeProgram(true))
	interpretable := interpreter.NewInterpretable(program)
	for i := 0; i < b.N; i++ {
		interpretable.Eval(emptyActivation)
	}
}

func BenchmarkInterpreter_ComprehensionExprWithInput(b *testing.B) {
	// elems.exists(x, type(x) == uint)
	program, _ := NewProgram(
		test.ExistsWithInput.Expr,
		test.ExistsWithInput.Info(b.Name()),
		OptimizeProgram(true))
	interpretable := interpreter.NewInterpretable(program)
	activation := NewActivation(map[string]interface{}{
		"elems": types.NativeToValue([]interface{}{0, 1, 2, 3, 4, uint(5), 6})})
	for i := 0; i < b.N; i++ {
		interpretable.Eval(activation)
	}
}

func BenchmarkInterpreter_CanonicalExpressions(b *testing.B) {
	for _, tst := range testData {
		s := common.NewTextSource(tst.E)
		parsed, errors := parser.Parse(s)
		if len(errors.GetErrors()) != 0 {
			b.Errorf(errors.ToDisplayString())
		}
		program, _ := NewProgram(
			parsed.GetExpr(),
			parsed.GetSourceInfo(),
			OptimizeProgram(true))
		interpretable := interpreter.NewInterpretable(program)
		activation := NewActivation(tst.I)
		b.Run(tst.name, func(bb *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < bb.N; i++ {
				interpretable.Eval(activation)
			}
		})
	}
}

var (
	interpreter = NewStandardInterpreter(
		packages.DefaultPackage,
		types.NewProvider(&exprpb.ParsedExpr{}))

	emptyActivation = NewActivation(map[string]interface{}{})

	testData = []testCase{
		{
			name: `ExprBench/ok_1st`,
			E:    `ai == 20 || ar["foo"] == "bar"`,
			I: map[string]interface{}{
				"ai": 20,
				"ar": map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			name: `ExprBench/ok_2nd`,
			E:    `ai == 20 || ar["foo"] == "bar"`,
			I: map[string]interface{}{
				"ai": 2,
				"ar": map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			name: `ExprBench/not_found`,
			E:    `ai == 20 || ar["foo"] == "bar"`,
			I: map[string]interface{}{
				"ai": 2,
				"ar": map[string]string{
					"foo": "baz",
				},
			},
		},
		{
			name: `ExprBench/false_1st`,
			E:    `false && true`,
			I:    map[string]interface{}{},
		},
		{
			name: `ExprBench/false_2nd`,
			E:    `true && false`,
			I:    map[string]interface{}{},
		},
		{
			name: `ExprBench/ok_lhs`,
			E:    `ai == 20 `,
			I: map[string]interface{}{
				"ai": 20,
				"ar": map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			name: `ExprBench/ok_rhs`,
			E:    `ar["foo"] == "bar"`,
			I: map[string]interface{}{
				"ai": 2,
				"ar": map[string]string{
					"foo": "baz",
				},
			},
		},
	}
)

func parseExpr(t *testing.T, src string) *exprpb.ParsedExpr {
	t.Helper()
	s := common.NewTextSource(src)
	parsed, errors := parser.Parse(s)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}
	return parsed
}

func evalExpr(t *testing.T, src string) (ref.Value, EvalState) {
	t.Helper()
	parsed := parseExpr(t, src)
	pgrm, _ := NewProgram(parsed.Expr, parsed.SourceInfo)
	eval := interpreter.NewInterpretable(pgrm)
	return eval.Eval(emptyActivation)
}

func compileExpr(t *testing.T, src string, decls ...*exprpb.Decl) *exprpb.CheckedExpr {
	t.Helper()
	s := common.NewTextSource(src)
	parsed, errors := parser.Parse(s)
	if len(errors.GetErrors()) != 0 {
		t.Error(errors.ToDisplayString())
		return nil
	}
	env := checker.NewStandardEnv(packages.DefaultPackage, types.NewProvider())
	env.Add(decls...)
	checked, errors := checker.Check(parsed, s, env)
	if len(errors.GetErrors()) != 0 {
		t.Error(errors.ToDisplayString())
		return nil
	}
	return checked
}
