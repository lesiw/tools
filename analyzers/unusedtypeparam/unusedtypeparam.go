// Package unusedtypeparam provides an analysis.Analyzer that reports a type
// parameter that a function, method, or type alias never uses.
//
// The Go spec says each name in a type parameter list declares "a new and
// different named type that acts as a placeholder for an (as of yet) unknown
// type in the declaration", and that the type parameter "is replaced with a
// type argument upon instantiation"
// (https://go.dev/ref/spec#Type_parameter_declarations). The scope of a
// function's type parameter begins after the function name and ends at the end
// of the body, and the scope of a type declaration's type parameter ends at
// the end of the TypeSpec
// (https://go.dev/ref/spec#Declarations_and_scope). Within that scope, a type
// parameter counts as used when it occurs in the parameters, results, body, or
// aliased type, or in the constraint of another type parameter that is used. A
// constraint does not use the parameter it constrains. A blank type parameter
// is not reported; the parameters its constraint refers to still count as
// used, because the slot remains.
//
// A type definition is left alone. The type argument is part of the
// instantiated type's identity (https://go.dev/ref/spec#Type_identity), so
// Tag[int] and Tag[string] are different types even when no field has type T.
// An alias denotes the same type as the one it aliases
// (https://go.dev/ref/spec#Alias_declarations), so an unused type parameter on
// an alias is reported.
//
// A method receiver is left alone too: the receiver type has one type
// parameter for each type parameter of the base type, and each appears in the
// receiver type (https://go.dev/ref/spec#Method_declarations). A type
// parameter list on the method itself is checked like a function's.
//
// A function with an empty body or a single statement that calls the builtin
// panic is a placeholder, so its type parameters are not reported.
//
// The rule is original to this suite and follows the Go spec's type parameter
// declarations and scope rules.
//
// Report only: blanking is illegal when a constraint refers to the name, and
// deletion changes the declaration and its callers or users. Either edit is
// left to the author.
package unusedtypeparam

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unusedtypeparam",
	Doc:  "reports a type parameter a function, method, or type alias never uses",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				checkFunc(pass, node)
			case *ast.TypeSpec:
				checkAlias(pass, node)
			}
			return true
		})
	}
	return nil, nil
}

// checkFunc reports unused type parameters on a function or method.
func checkFunc(pass *analysis.Pass, fn *ast.FuncDecl) {
	if fn.Type.TypeParams == nil || placeholder(pass, fn.Body) {
		return
	}
	var nodes []ast.Node
	nodes = append(nodes, fn.Type.Params)
	if fn.Type.Results != nil {
		nodes = append(nodes, fn.Type.Results)
	}
	nodes = append(nodes, fn.Body)
	live := liveParams(pass, fn.Type.TypeParams, nodes...)
	report(pass, fn.Type.TypeParams, live)
}

// checkAlias reports unused type parameters on a type alias.
func checkAlias(pass *analysis.Pass, spec *ast.TypeSpec) {
	if !spec.Assign.IsValid() || spec.TypeParams == nil {
		return
	}
	report(pass, spec.TypeParams, liveParams(pass, spec.TypeParams, spec.Type))
}

// liveParams returns the type parameters in list that a use under nodes
// requires, directly or through another live parameter's constraint.
func liveParams(pass *analysis.Pass, list *ast.FieldList, nodes ...ast.Node) map[types.Object]struct{} {
	declared := make(map[types.Object]struct{})
	for _, field := range list.List {
		for _, id := range field.Names {
			if obj := pass.TypesInfo.Defs[id]; obj != nil {
				declared[obj] = struct{}{}
			}
		}
	}
	live := usedParams(pass, declared, nodes...)
	for _, field := range list.List {
		for _, id := range field.Names {
			if id.Name != "_" {
				continue
			}
			if obj := pass.TypesInfo.Defs[id]; obj != nil {
				live[obj] = struct{}{}
			}
		}
	}
	deps := constraints(pass, list, declared)
	var queue []types.Object
	for obj := range live {
		queue = append(queue, obj)
	}
	for len(queue) > 0 {
		obj := queue[0]
		queue = queue[1:]
		for dep := range deps[obj] {
			if _, ok := live[dep]; ok {
				continue
			}
			live[dep] = struct{}{}
			queue = append(queue, dep)
		}
	}
	return live
}

// usedParams returns the type parameters in declared that nodes use.
func usedParams(pass *analysis.Pass, declared map[types.Object]struct{}, nodes ...ast.Node) map[types.Object]struct{} {
	out := make(map[types.Object]struct{})
	for _, node := range nodes {
		if node == nil {
			continue
		}
		ast.Inspect(node, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			obj := pass.TypesInfo.Uses[id]
			if _, ok := declared[obj]; ok {
				out[obj] = struct{}{}
			}
			return true
		})
	}
	return out
}

// constraints maps each type parameter in list to the type parameters its
// constraint uses.
func constraints(pass *analysis.Pass, list *ast.FieldList, declared map[types.Object]struct{}) map[types.Object]map[types.Object]struct{} {
	out := make(map[types.Object]map[types.Object]struct{})
	for _, field := range list.List {
		if field.Type == nil {
			continue
		}
		refs := usedParams(pass, declared, field.Type)
		for _, id := range field.Names {
			obj := pass.TypesInfo.Defs[id]
			if obj == nil {
				continue
			}
			deps := out[obj]
			if deps == nil {
				deps = make(map[types.Object]struct{})
				out[obj] = deps
			}
			for ref := range refs {
				deps[ref] = struct{}{}
			}
		}
	}
	return out
}

// report flags each type parameter in list that is not live.
func report(pass *analysis.Pass, list *ast.FieldList, live map[types.Object]struct{}) {
	for _, field := range list.List {
		for _, id := range field.Names {
			if id.Name == "_" {
				continue // blank type parameters need no report
			}
			obj := pass.TypesInfo.Defs[id]
			if obj == nil {
				continue
			}
			if _, ok := live[obj]; ok {
				continue
			}
			// No SuggestedFix: blanking is illegal when a constraint refers
			// to the name, and deletion changes its users.
			pass.Report(analysis.Diagnostic{
				Pos:     id.Pos(),
				End:     id.End(),
				Message: "type parameter " + id.Name + " is unused",
			})
		}
	}
}

// placeholder reports whether body is empty or a single statement that calls
// the builtin panic, which marks a function written as a stub.
func placeholder(pass *analysis.Pass, body *ast.BlockStmt) bool {
	if body == nil {
		return true // a malformed declaration; keep the analyzer safe
	}
	switch len(body.List) {
	case 0:
		return true
	case 1:
		if _, ok := body.List[0].(*ast.EmptyStmt); ok {
			return true
		}
		stmt, ok := body.List[0].(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := ast.Unparen(stmt.X).(*ast.CallExpr)
		if !ok {
			return false
		}
		fun, ok := ast.Unparen(call.Fun).(*ast.Ident)
		if !ok {
			return false
		}
		builtin, ok := pass.TypesInfo.Uses[fun].(*types.Builtin)
		return ok && builtin.Name() == "panic"
	}
	return false
}
