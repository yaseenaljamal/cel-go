package cel

import (
	"testing"

	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/types"
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
	attrNodeMap := make(map[string]attrNode, len(attrGraphNodes))
	for _, n := range attrGraphNodes {
		attrNodeMap[n.simpleName] = attrNode{
			attrEntry: n,
			attrAST:   testCseCompile(t, e, n.expr),
		}
	}

	seq := &attrSequence{
		nodeMap: attrNodeMap,
		ordered: []attrNode{},
	}
	sequenceNodes(attrNodeMap["root"], attrNodeMap, seq)
	mergedPB, err := ast.ToProto(seq.toAST())
	if err != nil {
		t.Fatalf("ast.ToProto() failed: %v", err)
	}
	t.Error(mergedPB)
}

type attrSequence struct {
	nodeMap map[string]attrNode
	ordered []attrNode
}

func (seq *attrSequence) add(node attrNode) {
	// reverse ordered nodes, such that root expression is inner-most when rendered to an AST
	seq.ordered = append([]attrNode{node}, seq.ordered...)
}

func (seq *attrSequence) toAST() *ast.AST {
	fac := ast.NewExprFactory()
	var currAST *ast.AST
	for _, node := range seq.ordered {
		nodeExpr := ast.NavigateAST(node.attrAST)
		for _, input := range node.inputs {
			inputNode := seq.nodeMap[input]
			matches := ast.MatchDescendants(nodeExpr, matchFullName(inputNode.fullName))
			for _, m := range matches {
				m.SetKindCase(fac.NewIdent(0, inputNode.simpleName))
			}
		}
		if currAST == nil {
			currAST = node.attrAST
			continue
		}
		fac.NewComprehension(0,
			fac.NewList(0, []ast.Expr{}, []int32{}),
			"#unused",
			node.simpleName,
			fac.CopyExpr(node.attrAST.Expr()),
			fac.NewLiteral(0, types.False),
			fac.NewIdent(0, node.simpleName),
			currAST.Expr())
	}
	return currAST
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

func sequenceNodes(node attrNode, nodeMap map[string]attrNode, seq *attrSequence) {
	if node.visited {
		return
	}
	node.visited = true
	// encountere a leaf node, early return
	if len(node.inputs) == 0 {
		seq.add(node)
		return
	}
	for _, inputName := range node.inputs {
		inputAttr := nodeMap[inputName]
		sequenceNodes(inputAttr, nodeMap, seq)
	}
	seq.add(node)
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
