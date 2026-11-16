// Package benchassign provides an analysis.Analyzer that reports an assignment
// to the N field of a testing.B.
//
// The testing package documents B (https://pkg.go.dev/testing#B) as "a type
// passed to Benchmark functions to manage benchmark timing and control the
// number of iterations". Its benchmark documentation
// (https://pkg.go.dev/testing) says the benchmark function is called multiple
// times with b.N adjusted until the run lasts long enough to be timed
// reliably. N has no doc comment; the benchmark section documents it. The
// testing package reports the measured duration against N, so an assignment to
// N makes the reported ns/op describe a different iteration count.
//
// The write may be a plain assignment, a compound assignment, an increment or
// decrement, or a range clause. A read of b.N is not reported, and neither is
// a field named N on another type. The rule reports a write whose receiver is
// a *testing.B, including a type alias or a defined pointer type, and a write
// promoted through embedded pointers to one. A struct that embeds testing.B by
// value holds a copy and is left alone. A selector on a value of type
// testing.B is left alone, as is one on a defined type whose underlying type
// is testing.B. Taking the address of such a value gives a pointer, and a
// write through it is reported. Writes in the testing package itself are not
// reported.
//
// Report only: only the author knows what count a benchmark should run, and
// removing the assignment changes what the benchmark measures.
//
// The rule is inspired by staticcheck's SA3001.
package benchassign

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "benchassign",
	Doc:  "reports an assignment to the N field of a testing.B",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Path() == "testing" {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				check(pass, n.Lhs)
			case *ast.IncDecStmt:
				check(pass, []ast.Expr{n.X})
			case *ast.RangeStmt:
				check(pass, []ast.Expr{n.Key, n.Value})
			}
			return true
		})
	}
	return nil, nil
}

// check reports each target in targets that is the N field of a B.
func check(pass *analysis.Pass, targets []ast.Expr) {
	for _, target := range targets {
		sel, ok := ast.Unparen(target).(*ast.SelectorExpr)
		if !ok || !isBenchN(pass, sel) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     sel.Pos(),
			End:     sel.End(),
			Message: "assignment to " + types.ExprString(sel),
		})
	}
}

// isBenchN reports whether sel names the N field of testing.B and the write
// reaches the benchmark through a *testing.B.
func isBenchN(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	field, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Var)
	if !ok || field != benchN(field.Pkg()) {
		return false
	}
	if star, ok := ast.Unparen(sel.X).(*ast.StarExpr); ok {
		if pointerTo(pass.TypesInfo.TypeOf(star.X), field) {
			return true
		}
	}
	seln := pass.TypesInfo.Selections[sel]
	if seln == nil {
		return false
	}
	typ := seln.Recv()
	if pointerTo(typ, field) {
		return true
	}
	if ptr, ok := pointerType(typ); ok {
		typ = ptr.Elem()
	}
	for _, index := range seln.Index() {
		for {
			ptr, ok := pointerType(typ)
			if !ok {
				break
			}
			typ = ptr.Elem()
		}
		st, ok := types.Unalias(typ).Underlying().(*types.Struct)
		if !ok || index < 0 || index >= st.NumFields() {
			return false
		}
		f := st.Field(index)
		if pointerTo(f.Type(), field) {
			return true
		}
		typ = f.Type()
	}
	return false
}

// pointerType returns the pointer underlying t, or false.
func pointerType(t types.Type) (*types.Pointer, bool) {
	t = types.Unalias(t)
	if named, ok := t.(*types.Named); ok {
		t = named.Underlying()
	}
	ptr, ok := t.(*types.Pointer)
	return ptr, ok
}

// pointerTo reports whether t is a pointer to a struct that has field.
func pointerTo(t types.Type, field *types.Var) bool {
	ptr, ok := pointerType(t)
	if !ok {
		return false
	}
	elem := types.Unalias(ptr.Elem())
	if named, ok := elem.(*types.Named); ok {
		elem = named.Underlying()
	}
	st, ok := elem.(*types.Struct)
	if !ok {
		return false
	}
	for f := range st.Fields() {
		if f == field {
			return true
		}
	}
	return false
}

// benchN returns the N field of testing.B in pkg, or nil.
func benchN(pkg *types.Package) *types.Var {
	if pkg == nil || pkg.Path() != "testing" {
		return nil
	}
	name, ok := pkg.Scope().Lookup("B").(*types.TypeName)
	if !ok {
		return nil
	}
	st, ok := name.Type().Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	for field := range st.Fields() {
		if field.Name() == "N" {
			return field
		}
	}
	return nil
}
