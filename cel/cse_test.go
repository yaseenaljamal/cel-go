package cel

import (
	"fmt"
	"testing"

	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/types"
	"google.golang.org/protobuf/encoding/prototext"
)

type attrEntry struct {
	fullName   string
	simpleName string
	expr       string
	inputs     []string
	celType    *Type
}

type attrNode struct {
	attrEntry
	attrAST *ast.AST
	visited bool
}

func TestSimpleCse(t *testing.T) {
	attrGraphNodes := []attrEntry{
		{
			fullName:   "token.end_system_spec",
			simpleName: "end_system_spec",
			expr:       "token.end_system_spec",
			inputs:     []string{},
			celType:    MapType(StringType, DynType),
		},
		{
			fullName:   "token.origin_product_ids",
			simpleName: "origin_product_ids",
			expr:       "token.end_system_spec.origin_product_ids",
			inputs: []string{
				"end_system_spec",
			},
			celType: MapType(StringType, ListType(IntType)),
		},
		{
			fullName:   "token.origin_consent_ids",
			simpleName: "origin_consent_ids",
			expr:       "lookupConsents(token.origin_product_ids)",
			inputs: []string{
				"origin_product_ids",
			},
			celType: MapType(StringType, ListType(IntType)),
		},
		{
			fullName:   "token.is_exempt",
			simpleName: "is_exempt",
			expr:       "token.end_system_spec.is_exempt",
			inputs: []string{
				"end_system_spec",
			},
			celType: MapType(StringType, ListType(IntType)),
		},
		{
			fullName:   "root",
			simpleName: "root",
			expr: `
				!token.is_exempt && 
				has(token.origin_consent_ids) && 
				123 in token.origin_consent_ids`,
			inputs: []string{
				"is_exempt",
				"origin_consent_ids",
			},
			celType: BoolType,
		},
	}
	e := testCseEnv(t,
		Variable("token", MapType(StringType, DynType)),
		Function("lookupConsents",
			Overload("lookupConsents_list", []*Type{ListType(IntType)}, ListType(IntType)),
		),
	)
	nextID := int64(1)
	idGen := func(int64) int64 {
		nextID++
		return nextID
	}
	attrNodeMap := make(map[string]*attrNode, len(attrGraphNodes))
	for _, n := range attrGraphNodes {
		attrAST := testCseCompile(t, e, n.expr)
		attrAST.Expr().RenumberIDs(idGen)
		attrNodeMap[n.simpleName] = &attrNode{
			attrEntry: n,
			attrAST:   attrAST,
		}
	}

	seq := &attrSequence{
		nodeMap: attrNodeMap,
		ordered: []*attrNode{},
		idGen:   idGen,
	}
	sequenceNodes(attrNodeMap["root"], attrNodeMap, seq)
	mergedPB, err := ast.ExprToProto(seq.toExpr())
	if err != nil {
		t.Fatalf("ast.ToProto() failed: %v", err)
	}
	t.Error(prototext.Format(mergedPB))
}

type attrSequence struct {
	nodeMap map[string]*attrNode
	ordered []*attrNode
	idGen   func(int64) int64
}

func (seq *attrSequence) add(node *attrNode) {
	// reverse ordered nodes, such that root expression is inner-most when rendered to an AST
	seq.ordered = append([]*attrNode{node}, seq.ordered...)
}

func (seq *attrSequence) toExpr() ast.Expr {
	fac := ast.NewExprFactory()
	var currExpr ast.Expr
	for _, node := range seq.ordered {
		nodeExpr := ast.NavigateAST(node.attrAST)
		for _, input := range node.inputs {
			inputNode := seq.nodeMap[input]
			matches := ast.MatchDescendants(nodeExpr, matchFullName(inputNode.fullName))
			for _, m := range matches {
				m.SetKindCase(fac.NewIdent(seq.nextID(), inputNode.simpleName))
			}
		}
		if currExpr == nil {
			currExpr = fac.CopyExpr(node.attrAST.Expr())
			continue
		}
		currExpr = fac.NewComprehension(seq.nextID(),
			fac.NewList(seq.nextID(), []ast.Expr{}, []int32{}),
			"#unused",
			node.simpleName,
			fac.CopyExpr(node.attrAST.Expr()),
			fac.NewLiteral(seq.nextID(), types.False),
			fac.NewIdent(seq.nextID(), node.simpleName),
			fac.CopyExpr(currExpr))
	}
	return currExpr
}

func (seq *attrSequence) nextID() int64 {
	return seq.idGen(0)
}

func matchFullName(fullName string) ast.ExprMatcher {
	return func(e ast.NavigableExpr) bool {
		if e.Kind() == ast.SelectKind {
			sel := e.AsSelect()
			// While the `ToQualifiedName` call could take the select directly, this
			// would skip presence tests from possible matches, which we would like
			// to include.
			qualName, found := containers.ToQualifiedName(sel.Operand())
			return found && qualName+"."+sel.FieldName() == fullName
		}
		return false
	}
}

func sequenceNodes(node *attrNode, nodeMap map[string]*attrNode, seq *attrSequence) {
	for _, inputName := range node.inputs {
		inputAttr := nodeMap[inputName]
		if inputAttr.visited {
			continue
		}
		sequenceNodes(inputAttr, nodeMap, seq)
	}
	if !node.visited {
		fmt.Println(node.fullName)
		node.visited = true
		seq.add(node)
	}
}

func testCseEnv(t *testing.T, opts ...EnvOption) *Env {
	t.Helper()
	e, err := NewEnv(opts...)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	return e
}

func testCseCompile(t *testing.T, e *Env, expr string) *ast.AST {
	t.Helper()
	ast, iss := e.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
	}
	return ast.NativeRep()
}
