// Package testhelpers provides an analysis.Analyzer that keeps tests on
// the testing.T-native version of an operation.
//
// In any function with a named *testing.T or *testing.B parameter:
//
//   - context.Background() and context.TODO() can be replaced by
//     t.Context().
//   - os.Setenv can be replaced by t.Setenv. The fix applies to a
//     bare call statement; a checked error needs its handling
//     removed by hand. A fix can leave the context or os import
//     unused; goimports cleans it up.
//   - os.MkdirTemp and os.Chdir can be replaced by t.TempDir and
//     t.Chdir; both change the error contract, so they are
//     reported without a fix.
//
// A function literal without its own *testing.T or *testing.B is
// not examined: in a t.Cleanup or goroutine body the test's context
// is already ending, so the replacements do not apply. Only
// package-qualified calls are matched.

package testhelpers

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "testhelpers",
	Doc:  "reports tests bypassing testing.T-native operations",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			var (
				ft   *ast.FuncType
				body *ast.BlockStmt
			)
			switch node := n.(type) {
			case *ast.FuncDecl:
				ft, body = node.Type, node.Body
			case *ast.FuncLit:
				ft, body = node.Type, node.Body
			default:
				return true
			}
			if body == nil {
				return true
			}
			t := testingParam(pass, ft)
			if t == "" {
				return true
			}
			checkBody(pass, t, body)
			return true
		})
	}
	return nil, nil
}

// testingParam returns the name of a *testing.T or *testing.B
// parameter, or empty.
func testingParam(pass *analysis.Pass, ft *ast.FuncType) string {
	for _, field := range ft.Params.List {
		typ := pass.TypesInfo.TypeOf(field.Type)
		if typ == nil {
			continue
		}
		s := typ.String()
		if s != "*testing.T" && s != "*testing.B" {
			continue
		}
		if len(field.Names) == 1 && field.Names[0].Name != "_" {
			return field.Names[0].Name
		}
	}
	return ""
}

// checkBody reports the T-native replacements within one function
// body. A nested literal is skipped: one with its own testing
// param is examined by its own visit, and one without is exempt.
func checkBody(pass *analysis.Pass, t string, body *ast.BlockStmt) {
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := callee(pass, call); fn {
		case "context.Background", "context.TODO":
			pass.Report(analysis.Diagnostic{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: fn + "() can be replaced by " + t + ".Context()",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Replace with " + t + ".Context()",
					TextEdits: []analysis.TextEdit{{
						Pos:     call.Pos(),
						End:     call.End(),
						NewText: []byte(t + ".Context()"),
					}},
				}},
			})
		case "os.Setenv":
			d := analysis.Diagnostic{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "os.Setenv can be replaced by " + t + ".Setenv",
			}
			if bareStatement(body, call) {
				d.SuggestedFixes = []analysis.SuggestedFix{{
					Message: "Replace with " + t + ".Setenv",
					TextEdits: []analysis.TextEdit{{
						Pos:     call.Fun.Pos(),
						End:     call.Fun.End(),
						NewText: []byte(t + ".Setenv"),
					}},
				}}
			}
			pass.Report(d)
		case "os.MkdirTemp":
			pass.Reportf(call.Pos(), "os.MkdirTemp can be replaced by %s.TempDir", t)
		case "os.Chdir":
			pass.Reportf(call.Pos(), "os.Chdir can be replaced by %s.Chdir", t)
		}
		return true
	})
}

// bareStatement reports whether call is an expression statement of
// the body: its result is discarded, so a name swap is the whole
// fix.
func bareStatement(body *ast.BlockStmt, call *ast.CallExpr) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		es, ok := n.(*ast.ExprStmt)
		if ok && es.X == call {
			found = true
		}
		return !found
	})
	return found
}

// callee returns the package-qualified name of a called package
// function.
func callee(pass *analysis.Pass, call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	pkg, ok := pass.TypesInfo.Uses[id].(*types.PkgName)
	if !ok {
		return ""
	}
	return pkg.Imported().Path() + "." + sel.Sel.Name
}
