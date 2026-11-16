// Package errorsisorder provides an analysis.Analyzer that reports a call to
// errors.Is whose first argument is a package-level error variable from
// another package.
//
// The errors package documents Is: "Is reports whether any error in err's
// tree matches target." The first argument is the error being checked and
// the second is the error to match. A leaf sentinel with no Is method in the
// first position makes the call match only the sentinel itself, since it
// wraps nothing.
//
// A package-level error variable can also wrap another error or have an Is
// method. The original call can then be correct, and the swap can reverse its
// result. To keep an intentional call, bind the package-level error to a local
// variable first; a local first argument is left alone. The suggested fix
// swaps the arguments; the diagnostic message is "first argument is a
// package-level error; swapping changes behavior".
//
// The first argument has to be a package-level variable from another package.
// The variable may be nil or assigned at run time; only its declaration
// matters. A first argument that is a local variable, a field, a call, a
// constant, or a composite literal is left alone because none is a
// package-level variable. A call through a function value is left alone too;
// only a direct call to errors.Is is matched. A first argument that is a
// package-level variable from the package being analyzed is left alone. A
// call whose two arguments are both package-level variables from other
// packages is left alone too: comparing two package-level errors from other
// packages is a legitimate use.
//
// The fix swaps the arguments, drops parentheses around either argument, and
// swaps the order in which they are evaluated. It is withheld when a comment
// lies in the argument list. The suggested fix message includes the caveat.
//
// The rule is adapted from staticcheck's SA1032, which reports only; this rule
// adds the swap.
package errorsisorder

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "errorsisorder",
	Doc:  "reports a call to errors.Is whose first argument is a package-level error variable from another package",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports a call to errors.Is that puts an external package-level error
// variable first, unless the second argument is also an external package-level
// error variable.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) {
	if len(call.Args) != 2 || !isErrorsIs(pass, call.Fun) {
		return
	}
	first, ok := packageVar(pass, call.Args[0])
	if !ok || first.Pkg() == pass.Pkg {
		return
	}
	if second, ok := packageVar(pass, call.Args[1]); ok {
		if second.Pkg() != pass.Pkg {
			return
		}
	}
	d := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "first argument is a package-level error; swapping changes behavior",
	}
	if !commented(file, call) {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Swap the arguments (changes behavior)",
			TextEdits: []analysis.TextEdit{{
				Pos:     call.Args[0].Pos(),
				End:     call.Args[0].End(),
				NewText: []byte(astexpr.Text(ast.Unparen(call.Args[1]))),
			}, {
				Pos:     call.Args[1].Pos(),
				End:     call.Args[1].End(),
				NewText: []byte(astexpr.Text(ast.Unparen(call.Args[0]))),
			}},
		}}
	}
	pass.Report(d)
}

// isErrorsIs reports whether fun is errors.Is.
func isErrorsIs(pass *analysis.Pass, fun ast.Expr) bool {
	ident := identOf(fun)
	if ident == nil {
		return false
	}
	fn, ok := pass.TypesInfo.Uses[ident].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "errors" {
		return false
	}
	return fn.Signature().Recv() == nil && fn.Name() == "Is"
}

// packageVar returns the package-level variable that e refers to.
func packageVar(pass *analysis.Pass, e ast.Expr) (*types.Var, bool) {
	ident := identOf(e)
	if ident == nil {
		return nil, false
	}
	obj, ok := pass.TypesInfo.Uses[ident].(*types.Var)
	if !ok || obj.Pkg() == nil || obj.Parent() != obj.Pkg().Scope() {
		return nil, false
	}
	return obj, true
}

// identOf returns the identifier that e refers to, through parentheses and a
// selector.
func identOf(e ast.Expr) *ast.Ident {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	}
	return nil
}

// commented reports whether a comment lies inside the argument list of call.
func commented(file *ast.File, call *ast.CallExpr) bool {
	for _, group := range file.Comments {
		if group.Pos() < call.Rparen && group.End() > call.Lparen {
			return true
		}
	}
	return false
}
