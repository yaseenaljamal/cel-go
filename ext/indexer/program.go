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
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter"
)

type IndexedProgram struct {
	env         *cel.Env
	idxAST      *IndexedAST
	maskTests   []interpreter.Attribute
	idxPrograms []cel.Program
}

func NewIndexedProgram(env *cel.Env, idxAST *IndexedAST) (*IndexedProgram, error) {
	types := env.CELTypeProvider()
	attrFactory := interpreter.NewAttributeFactory(env.Container, env.CELTypeAdapter(), types)
	maskTests := make([]interpreter.Attribute, len(idxAST.Fields))
	for i, f := range idxAST.Fields {
		path := f.FieldPath()
		if len(path) < 2 {
			continue
		}
		v := path[0]
		varDecl, found := env.FindVariable(v)
		if !found {
			continue
		}
		var attr interpreter.Attribute = attrFactory.AbsoluteAttribute(0, varDecl.Name())
		objType := varDecl.Type()
		for _, p := range path[1:] {
			q, err := attrFactory.NewQualifier(objType, 0, p, true)
			if err != nil {
				return nil, err
			}
			attr, err = attr.AddQualifier(q)
			if err != nil {
				return nil, err
			}
			if ft, found := types.FindStructFieldType(objType.TypeName(), p); found {
				objType = ft.Type
			} else {
				objType = cel.DynType
			}
		}
		maskTests[i] = attr
	}

	idxPrograms := make([]cel.Program, len(idxAST.ASTs))
	for i, a := range idxAST.ASTs {
		prg, err := env.Program(a)
		if err != nil {
			return nil, err
		}
		idxPrograms[i] = prg
	}

	return &IndexedProgram{
		env:         env,
		maskTests:   maskTests,
		idxAST:      idxAST,
		idxPrograms: idxPrograms,
	}, nil
}

func (prg *IndexedProgram) Eval(vars any) (ref.Val, *cel.EvalDetails, error) {
	act, err := interpreter.NewActivation(vars)
	if err != nil {
		return nil, nil, err
	}
	mask := uint8(0)
	for bit, maskTest := range prg.maskTests {
		v, err := maskTest.Resolve(act)
		if err != nil {
			return nil, nil, err
		}
		if v == types.OptionalNone {
			continue
		}
		mask = mask | (1 << bit)
	}
	idx := prg.idxAST.MaskToASTSlot[mask]
	idxPrg := prg.idxPrograms[idx]
	return idxPrg.Eval(vars)
}
