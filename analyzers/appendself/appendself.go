// Package appendself provides an analysis.Analyzer that reports an append
// result assigned to a slice other than the one it appends to.
//
// The spec defines append and its result
// (https://go.dev/ref/spec#Appending_and_copying_slices). dst = append(src, v)
// stores a slice holding src's elements followed by v in dst. The previous
// elements of dst are absent from the result.
//
// The rule reports an append whose source the function does not read again
// and whose destination the function does read again. appendassign reports
// the other case, where the source is read again and its length does not
// change. The rule reports only when the function reads the destination
// again. A later write to the destination suppresses the report.
//
// Report only: replacing src with dst changes the result, and a local src can
// become unused, so the analyzer reports without a fix.
//
// The rule is adapted from staticcheck's SA4021, which reports an append that
// passes only the slice and replaces it with the slice. This rule covers one
// appended value and reports only, because changing the source changes the
// result.
package appendself

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "appendself",
	Doc:  "reports an append result assigned to a different slice",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if assign, ok := n.(*ast.AssignStmt); ok {
				body, results := within(stack)
				check(pass, body, results, assign)
			}
			stack = append(stack, n)
			return true
		})
	}
	return nil, nil
}

// within returns the body and results of the innermost function the stack is
// inside.
func within(stack []ast.Node) (*ast.BlockStmt, *ast.FieldList) {
	for _, n := range slices.Backward(stack) {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			return fn.Body, fn.Type.Results
		case *ast.FuncLit:
			return fn.Body, fn.Type.Results
		}
	}
	return nil, nil
}

func check(pass *analysis.Pass, body *ast.BlockStmt, results *ast.FieldList, assign *ast.AssignStmt) {
	if assign.Tok != token.ASSIGN || len(assign.Lhs) != len(assign.Rhs) {
		return
	}
	for i, rhs := range assign.Rhs {
		dst, ok := ident(assign.Lhs[i])
		if !ok {
			continue
		}
		call, ok := appendCall(pass, rhs)
		if !ok {
			continue
		}
		src, ok := ident(call.Args[0])
		if !ok {
			continue
		}
		dstObj := pass.TypesInfo.ObjectOf(dst)
		if !sliceType(pass.TypesInfo.TypeOf(dst)) {
			continue
		}
		srcObj := pass.TypesInfo.ObjectOf(src)
		if srcObj == dstObj {
			continue
		}
		if reads(pass, body, results, assign.End(), srcObj) {
			continue
		}
		if !readsDestination(pass, body, results, assign.End(), dstObj) {
			continue
		}
		if rangeValue(pass, body, assign.Pos(), srcObj) {
			continue
		}
		related := derivedFrom(pass, body, assign.Pos(), srcObj, dstObj)
		related = related || derivedFrom(
			pass, body, assign.Pos(),
			dstObj, srcObj,
		)
		if related {
			continue
		}
		pass.Reportf(
			rhs.Pos(),
			"append to %s is assigned to %s; the result does not extend %s",
			src.Name, dst.Name, dst.Name,
		)
	}
}

// ident returns e as an identifier that names something other than the blank.
func ident(e ast.Expr) (*ast.Ident, bool) {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok || id.Name == "_" {
		return nil, false
	}
	return id, true
}

// sliceType reports whether t is a slice or a type parameter whose type set
// holds slice types.
func sliceType(t types.Type) bool {
	if t == nil {
		return false
	}
	switch u := t.Underlying().(type) {
	case *types.Slice:
		return true
	case *types.Interface:
		return sliceSet(u)
	}
	return false
}

// sliceSet reports whether the type set of iface holds only slices.
func sliceSet(iface *types.Interface) bool {
	if iface.NumEmbeddeds() != 1 {
		return false
	}
	embed := iface.EmbeddedType(0)
	if union, ok := embed.(*types.Union); ok {
		for term := range union.Terms() {
			if _, ok := term.Type().Underlying().(*types.Slice); !ok {
				return false
			}
		}
		return union.Len() > 0
	}
	_, ok := embed.Underlying().(*types.Slice)
	return ok
}

// reads reports whether body reads obj after pos, including through a naked
// return that reads a named result.
func reads(pass *analysis.Pass, body *ast.BlockStmt, results *ast.FieldList, pos token.Pos, obj types.Object) bool {
	if body == nil {
		return false
	}
	return readAgain(pass, body, pos, obj) ||
		nakedReturn(pass, body, results, pos, obj)
}

// readsDestination reports whether body reads obj after pos, where the append
// result reaches the read. A later write to obj suppresses the report, because
// a later read may see another value.
func readsDestination(pass *analysis.Pass, body *ast.BlockStmt, results *ast.FieldList, pos token.Pos, obj types.Object) bool {
	if body == nil || writtenLater(pass, body, pos, obj) {
		return false
	}
	return reads(pass, body, results, pos, obj)
}

// writtenLater reports whether body writes obj after pos without reading it
// in the same statement. A statement that reads obj still reads the append
// result, as dst = append(dst, w) does.
func writtenLater(pass *analysis.Pass, body *ast.BlockStmt, pos token.Pos, obj types.Object) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found || n == nil {
			return false
		}
		stmt, ok := n.(ast.Stmt)
		if !ok || stmt.Pos() <= pos || !assigns(pass, stmt, obj) {
			return true
		}
		if assign, ok := stmt.(*ast.AssignStmt); ok {
			for _, rhs := range assign.Rhs {
				if mentions(pass, rhs, obj) {
					return true
				}
			}
		}
		found = true
		return false
	})
	return found
}

// mentions reports whether n reads obj.
func mentions(pass *analysis.Pass, n ast.Node, obj types.Object) bool {
	var found bool
	ast.Inspect(n, func(n ast.Node) bool {
		if found {
			return false
		}
		id, ok := n.(*ast.Ident)
		if ok && pass.TypesInfo.ObjectOf(id) == obj {
			found = true
		}
		return !found
	})
	return found
}

// nakedReturn reports whether a naked return after pos reads obj as a named
// result.
func nakedReturn(pass *analysis.Pass, body *ast.BlockStmt, results *ast.FieldList, pos token.Pos, obj types.Object) bool {
	if results == nil || !declares(pass, results, obj) {
		return false
	}
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		ret, ok := n.(*ast.ReturnStmt)
		if ok && ret.Pos() > pos && len(ret.Results) == 0 {
			found = true
			return false
		}
		return true
	})
	return found
}

func declares(pass *analysis.Pass, results *ast.FieldList, obj types.Object) bool {
	for _, field := range results.List {
		for _, name := range field.Names {
			if pass.TypesInfo.ObjectOf(name) == obj {
				return true
			}
		}
	}
	return false
}

// readAgain reports whether body reads obj after pos.
func readAgain(pass *analysis.Pass, body *ast.BlockStmt, pos token.Pos, obj types.Object) bool {
	var (
		found bool
		stack []ast.Node
		later = func(id *ast.Ident) bool {
			return id.Pos() > pos && !written(stack, id) &&
				pass.TypesInfo.ObjectOf(id) == obj
		}
	)
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if id, ok := n.(*ast.Ident); ok && later(id) {
			found = true
			return false
		}
		stack = append(stack, n)
		return true
	})
	return found
}

// assigns reports whether stmt writes obj.
func assigns(pass *analysis.Pass, stmt ast.Stmt, obj types.Object) bool {
	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		for _, lhs := range stmt.Lhs {
			id, ok := ast.Unparen(lhs).(*ast.Ident)
			if ok && pass.TypesInfo.ObjectOf(id) == obj {
				return true
			}
		}
	case *ast.RangeStmt:
		for _, expr := range []ast.Expr{stmt.Key, stmt.Value} {
			id, ok := ast.Unparen(expr).(*ast.Ident)
			if ok && pass.TypesInfo.ObjectOf(id) == obj {
				return true
			}
		}
	}
	return false
}

// written reports whether id is a target of the statement that holds it.
func written(stack []ast.Node, id *ast.Ident) bool {
	for _, n := range slices.Backward(stack) {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			return slices.ContainsFunc(stmt.Lhs, func(e ast.Expr) bool {
				return ast.Unparen(e) == ast.Expr(id)
			})
		case *ast.RangeStmt:
			return ast.Unparen(stmt.Key) == ast.Expr(id) ||
				ast.Unparen(stmt.Value) == ast.Expr(id)
		}
	}
	return false
}

// appendCall returns e as a call to the builtin append that adds exactly one
// value. A spread append joins a second slice, which the rule does not cover.
func appendCall(pass *analysis.Pass, e ast.Expr) (*ast.CallExpr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 2 || call.Ellipsis != token.NoPos {
		return nil, false
	}
	builtin, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[builtin] != types.Universe.Lookup("append") {
		return nil, false
	}
	return call, true
}

// rangeValue reports whether a range clause that contains pos assigns obj,
// which gives the source a new value on every pass.
func rangeValue(pass *analysis.Pass, body *ast.BlockStmt, pos token.Pos, obj types.Object) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		rangeStmt, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if pos < rangeStmt.Pos() || pos > rangeStmt.End() {
			return true
		}
		for _, expr := range []ast.Expr{rangeStmt.Key, rangeStmt.Value} {
			id, ok := ast.Unparen(expr).(*ast.Ident)
			if ok && id.Name != "_" && pass.TypesInfo.ObjectOf(id) == obj {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// derivedFrom reports whether the body writes src from an expression that
// mentions dst before pos. The two names may share an array, so the append may
// extend the destination.
func derivedFrom(pass *analysis.Pass, body *ast.BlockStmt, pos token.Pos, src, dst types.Object) bool {
	var (
		found    bool
		mentions = func(n ast.Node) bool {
			var seen bool
			ast.Inspect(n, func(n ast.Node) bool {
				if seen {
					return false
				}
				id, ok := n.(*ast.Ident)
				if ok && pass.TypesInfo.ObjectOf(id) == dst {
					seen = true
				}
				return !seen
			})
			return seen
		}
	)
	ast.Inspect(body, func(n ast.Node) bool {
		if found || n == nil || n.Pos() >= pos {
			return false
		}
		switch n := n.(type) {
		case *ast.AssignStmt:
			if len(n.Lhs) != len(n.Rhs) {
				return true
			}
			for i, lhs := range n.Lhs {
				id, ok := ast.Unparen(lhs).(*ast.Ident)
				if !ok || pass.TypesInfo.ObjectOf(id) != src {
					continue
				}
				if mentions(n.Rhs[i]) {
					found = true
					return false
				}
			}
		case *ast.ValueSpec:
			if len(n.Names) != len(n.Values) {
				return true
			}
			for i, name := range n.Names {
				if pass.TypesInfo.ObjectOf(name) != src {
					continue
				}
				if mentions(n.Values[i]) {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}
