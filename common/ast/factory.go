package ast

import "github.com/google/cel-go/common/types/ref"

type ExprFactory interface {
	NewCall(id int64, function string, args ...Expr) Expr

	NewComprehension(id int64, iterRange Expr, iterVar, accuVar string, accuInit, loopCondition, loopStep, result Expr) Expr

	NewMemberCall(id int64, function string, receiver Expr, args ...Expr) Expr

	NewIdent(id int64, name string) Expr

	NewAccuIdent(id int64) Expr

	NewLiteral(id int64, value ref.Val) Expr

	NewList(id int64, elems []Expr, optIndices []int32) Expr

	NewMap(id int64, entries []EntryExpr) Expr

	NewMapEntry(id int64, key, value Expr, isOptional bool) EntryExpr

	NewPresenceTest(id int64, operand Expr, field string) Expr

	NewSelect(id int64, operand Expr, field string) Expr

	NewStruct(id int64, typeName string, fields []EntryExpr) Expr

	NewStructField(id int64, field string, value Expr, isOptional bool) EntryExpr

	NewUnspecifiedExpr(id int64) Expr

	isExprFactory()
}

type baseExprFactory struct{}

func NewExprFactory() ExprFactory {
	return &baseExprFactory{}
}

func (fac *baseExprFactory) NewCall(id int64, function string, args ...Expr) Expr {
	return fac.newExpr(
		id,
		&baseCallExpr{
			function: function,
			target:   nil,
			args:     args,
			isMember: false,
		})
}

func (fac *baseExprFactory) NewMemberCall(id int64, function string, target Expr, args ...Expr) Expr {
	return fac.newExpr(
		id,
		&baseCallExpr{
			function: function,
			target:   target,
			args:     args,
			isMember: true,
		})
}

func (fac *baseExprFactory) NewComprehension(id int64, iterRange Expr, iterVar, accuVar string, accuInit, loopCond, loopStep, result Expr) Expr {
	return fac.newExpr(
		id,
		&baseComprehensionExpr{
			iterRange: iterRange,
			iterVar:   iterVar,
			accuVar:   accuVar,
			accuInit:  accuInit,
			loopCond:  loopCond,
			loopStep:  loopStep,
			result:    result,
		})
}

func (fac *baseExprFactory) NewIdent(id int64, name string) Expr {
	return fac.newExpr(id, baseIdentExpr(name))
}

func (fac *baseExprFactory) NewAccuIdent(id int64) Expr {
	return fac.NewIdent(id, "__result__")
}

func (fac *baseExprFactory) NewLiteral(id int64, value ref.Val) Expr {
	return fac.newExpr(id, &baseLiteral{Val: value})
}

func (fac *baseExprFactory) NewList(id int64, elems []Expr, optIndices []int32) Expr {
	return fac.newExpr(id, &baseListExpr{elements: elems, optIndices: optIndices})
}

func (fac *baseExprFactory) NewMap(id int64, entries []EntryExpr) Expr {
	return fac.newExpr(id, &baseMapExpr{entries: entries})
}

func (fac *baseExprFactory) NewMapEntry(id int64, key, value Expr, isOptional bool) EntryExpr {
	return fac.newEntryExpr(
		id,
		&baseMapEntry{
			key:        key,
			value:      value,
			isOptional: isOptional,
		})
}

func (fac *baseExprFactory) NewPresenceTest(id int64, operand Expr, field string) Expr {
	return fac.newExpr(
		id,
		&baseSelectExpr{
			operand:  operand,
			field:    field,
			testOnly: true,
		})
}

func (fac *baseExprFactory) NewSelect(id int64, operand Expr, field string) Expr {
	return fac.newExpr(
		id,
		&baseSelectExpr{
			operand: operand,
			field:   field,
		})
}

func (fac *baseExprFactory) NewStruct(id int64, typeName string, fields []EntryExpr) Expr {
	return fac.newExpr(
		id,
		&baseStructExpr{
			typeName: typeName,
			fields:   fields,
		})
}

func (fac *baseExprFactory) NewStructField(id int64, field string, value Expr, isOptional bool) EntryExpr {
	return fac.newEntryExpr(
		id,
		&baseStructField{
			field:      field,
			value:      value,
			isOptional: isOptional,
		})
}

func (fac *baseExprFactory) NewUnspecifiedExpr(id int64) Expr {
	return fac.newExpr(id, nil)
}

func (*baseExprFactory) isExprFactory() {}

func (fac *baseExprFactory) newExpr(id int64, e exprKindCase) Expr {
	return &expr{
		id:           id,
		exprKindCase: e,
	}
}

func (fac *baseExprFactory) newEntryExpr(id int64, e entryExprKindCase) EntryExpr {
	return &entryExpr{
		id:                id,
		entryExprKindCase: e,
	}
}
