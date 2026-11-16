// Package writebytes provides an analysis.Analyzer that reports a byte slice
// converted to a string for io.WriteString.
//
// io.WriteString's doc says that it calls Write "exactly once" when the
// writer's type does not implement io.StringWriter
// (https://pkg.go.dev/io#WriteString), and Write takes the byte slice
// directly. Converting the slice to a string first copies every byte, so
// w.Write(b) writes the same bytes without the copy when Write is the method
// that runs.
// The rewrite is valid because io.Writer's doc requires that Write not modify
// or retain the slice. The fix is offered only when b cannot be nil, since a
// nil b passed to Write may differ from the non-nil empty slice io.WriteString
// passes.
//
// The call is reported when the second argument is string(b), b is assignable
// to []byte, and the writer is concrete and does not implement
// io.StringWriter. The fix replaces the call with w.Write(b). It is withheld
// where a comment sits in the text it would replace, where the call is the
// direct call of a defer or go statement and its arguments are evaluated
// before it runs, where the operand may be nil, and where the fixes together
// would leave a dot import unused. The README catalogs every check, exemption,
// and fix.
//
// The rule is adapted from staticcheck's SA6006, which reports the call for
// any writer and offers no fix.
package writebytes

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "writebytes",
	Doc:  "reports a byte slice converted to a string for io.WriteString",
	Run:  run,
}

var (
	byteSlice = types.NewSlice(types.Typ[types.Byte])
	errorTyp  = types.Universe.Lookup("error").Type()
	intTyp    = types.Typ[types.Int]
	stringTyp = types.Typ[types.String]
)

type site struct {
	call *ast.CallExpr
	b    ast.Expr
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var sites []site
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			if !writeString(pass, call.Fun) {
				return true
			}
			b, ok := converted(pass, call.Args[1])
			if !ok || !reportable(pass, call.Args[0]) {
				return true
			}
			sites = append(sites, site{call, b})
			return true
		})
		delayed := delayedCalls(file)
		candidates := make([]bool, len(sites))
		for i, s := range sites {
			candidates[i] = !commented(file, s.call.Pos(), s.call.End()) &&
				!delayed[s.call] && nonNil(pass, s.b)
		}
		dotUnused := imports.Names(pass, file)["io"] == "." &&
			!otherIOUse(pass, file, sites, candidates)
		for i, s := range sites {
			var (
				known   = nonNil(pass, s.b)
				fixable = candidates[i] && !dotUnused
				diag    = analysis.Diagnostic{
					Pos:     s.call.Pos(),
					End:     s.call.End(),
					Message: "byte slice converted to a string for io.WriteString",
				}
			)
			if !delayed[s.call] && known {
				diag.Message += "; use Write"
			}
			if fixable {
				diag.SuggestedFixes = []analysis.SuggestedFix{{
					Message: "Call Write",
					TextEdits: []analysis.TextEdit{{
						Pos:     s.call.Pos(),
						End:     s.call.End(),
						NewText: []byte(rewrite(s.call, s.b)),
					}},
				}}
			}
			pass.Report(diag)
		}
	}
	return nil, nil
}

// writeString reports whether fun is io.WriteString.
func writeString(pass *analysis.Pass, fun ast.Expr) bool {
	var ident *ast.Ident
	switch fun := ast.Unparen(fun).(type) {
	case *ast.SelectorExpr:
		ident = fun.Sel
	case *ast.Ident:
		ident = fun
	default:
		return false
	}
	fn, ok := pass.TypesInfo.Uses[ident].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() == nil &&
		fn.Pkg().Path() == "io" && fn.Name() == "WriteString"
}

// converted returns the operand of a conversion to the predeclared string
// type, when the operand is assignable to []byte.
func converted(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return nil, false
	}
	if !types.Identical(tv.Type, stringTyp) {
		return nil, false
	}
	b := ast.Unparen(call.Args[0])
	t := pass.TypesInfo.TypeOf(b)
	if t == nil || !types.AssignableTo(t, byteSlice) {
		return nil, false
	}
	return b, true
}

// rewrite returns the call with the writer's Write method in place of
// io.WriteString and the byte slice in place of the string conversion.
func rewrite(call *ast.CallExpr, b ast.Expr) string {
	return astexpr.Text(&ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   call.Args[0],
			Sel: ast.NewIdent("Write"),
		},
		Args: []ast.Expr{b},
	})
}

// nonNil reports whether e always yields a non-nil byte slice.
func nonNil(pass *analysis.Pass, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.CompositeLit:
		t := pass.TypesInfo.TypeOf(e)
		return t != nil && slice(t)
	case *ast.CallExpr:
		tv, ok := pass.TypesInfo.Types[e.Fun]
		if ok && tv.IsType() && slice(tv.Type) && len(e.Args) == 1 {
			t := pass.TypesInfo.TypeOf(e.Args[0])
			return t != nil && types.AssignableTo(t, stringTyp)
		}
		id, ok := ast.Unparen(e.Fun).(*ast.Ident)
		if !ok || pass.TypesInfo.Uses[id] != types.Universe.Lookup("make") {
			return false
		}
		if len(e.Args) == 0 {
			return false
		}
		t := pass.TypesInfo.TypeOf(e.Args[0])
		return t != nil && slice(t)
	}
	return false
}

// slice reports whether t is a slice type.
func slice(t types.Type) bool {
	_, ok := t.Underlying().(*types.Slice)
	return ok
}

// reportable reports whether the writer's type is concrete and has no
// WriteString, so io.WriteString calls Write and the conversion is
// unnecessary.
func reportable(pass *analysis.Pass, writer ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(writer)
	return t != nil && !pass.TypesInfo.Types[writer].IsNil() &&
		!open(t) && !hasWriteString(pass, writer)
}

// delayedCalls returns the calls whose arguments a defer or go statement
// evaluates before the call runs.
func delayedCalls(file *ast.File) map[*ast.CallExpr]bool {
	out := make(map[*ast.CallExpr]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.DeferStmt:
			out[stmt.Call] = true
		case *ast.GoStmt:
			out[stmt.Call] = true
		}
		return true
	})
	return out
}

// otherIOUse reports whether an object from io appears outside the calls a fix
// removes, so a dot import of io stays used.
func otherIOUse(pass *analysis.Pass, file *ast.File, sites []site, candidates []bool) bool {
	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := pass.TypesInfo.Uses[id]
		if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "io" {
			return true
		}
		if !removed(id, sites, candidates) {
			found = true
			return false
		}
		return true
	})
	return found
}

// removed reports whether a fix deletes the identifier, which lies in a call
// but outside its arguments.
func removed(id *ast.Ident, sites []site, candidates []bool) bool {
	for i, s := range sites {
		if !candidates[i] {
			continue
		}
		if id.Pos() < s.call.Pos() || id.End() > s.call.End() {
			continue
		}
		for _, arg := range s.call.Args {
			if arg.Pos() <= id.Pos() && id.End() <= arg.End() {
				return false
			}
		}
		return true
	}
	return false
}

// open reports whether the type of a value can gain methods at run time.
func open(t types.Type) bool {
	if _, ok := t.(*types.TypeParam); ok {
		return true
	}
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// hasWriteString reports whether the writer has a WriteString method with the
// signature io.StringWriter declares.
func hasWriteString(pass *analysis.Pass, writer ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(writer)
	if t == nil {
		return true
	}
	obj, _, _ := types.LookupFieldOrMethod(t, false, pass.Pkg, "WriteString")
	fn, ok := obj.(*types.Func)
	return ok && writeStringSignature(fn)
}

// writeStringSignature reports whether fn has io.StringWriter's WriteString
// signature.
func writeStringSignature(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Variadic() || sig.Params().Len() != 1 {
		return false
	}
	if sig.Results().Len() != 2 {
		return false
	}
	return types.Identical(sig.Params().At(0).Type(), stringTyp) &&
		types.Identical(sig.Results().At(0).Type(), intTyp) &&
		types.Identical(sig.Results().At(1).Type(), errorTyp)
}

// commented reports whether a comment sits in the span a fix would replace.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
