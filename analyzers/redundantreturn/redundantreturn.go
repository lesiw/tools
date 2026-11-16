// Package redundantreturn provides an analysis.Analyzer that reports a return
// statement with no results at the end of a function that has no results.
//
// The function declarations section of the spec requires a body's statement
// list to end in a terminating statement only when the signature declares
// result parameters
// (https://go.dev/ref/spec#Function_declarations). The Defer statements
// section says a function also returns by reaching the end of its body
// (https://go.dev/ref/spec#Defer_statements). A function without a result type
// returns when its last statement finishes, with or without a trailing return,
// so the return has no effect.
//
// Deleting a return that another statement follows would let that statement
// run, so only the statement that ends the body is reported. A return nested
// in a final if, switch, select, or block is left alone, as in S1023, and a
// return inside a loop is left alone because deleting it would let the loop
// run again. A function literal is a function, so a trailing return in one is
// reported. A function that declares result parameters is left alone. A
// result list with no parameters declares no results, so that form is
// reported. A labeled return is not reported: the labeled statement is last,
// and deleting the return would leave the label without a statement.
//
// A generated file, including cgo output, is left alone, since an edit would
// change generated text instead of its source.
//
// Safe fix: delete the return. The fix removes its line when the return is
// alone on it and the line ends with a newline, and otherwise removes the
// return and a semicolon that follows it after spaces or tabs, and leaves the
// rest of the line.
//
// Adapted from the return check in staticcheck's S1023.
package redundantreturn

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "redundantreturn",
	Doc:  "reports a return with no results at the end of a function that has no results",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// The suite filters generated files by position. A //line directive
		// in cgo output changes a diagnostic's filename, so the filter does
		// not cover it.
		if ast.IsGenerated(file) {
			continue
		}
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			var (
				typ  *ast.FuncType
				body *ast.BlockStmt
			)
			switch fn := n.(type) {
			case *ast.FuncDecl:
				typ, body = fn.Type, fn.Body
			case *ast.FuncLit:
				typ, body = fn.Type, fn.Body
			default:
				return true
			}
			if typ.Results != nil && len(typ.Results.List) > 0 {
				return true
			}
			if body == nil || len(body.List) == 0 {
				return true
			}
			ret, ok := body.List[len(body.List)-1].(*ast.ReturnStmt)
			// A well-typed pass cannot reach this with results, but a
			// gopls-style driver analyzes packages with type errors.
			if !ok || len(ret.Results) != 0 {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     ret.Pos(),
				End:     ret.End(),
				Message: "return statement at the end of a function with no results has no effect",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message:   "Delete the return",
					TextEdits: []analysis.TextEdit{removal(pass, src, ret)},
				}},
			})
			return true
		})
	}
	return nil, nil
}

// removal returns the edit that deletes ret. When the return is alone on its
// line and the line ends with a newline, the fix deletes the whole line;
// otherwise it deletes the return and a semicolon that follows it after spaces
// or tabs.
func removal(pass *analysis.Pass, src []byte, ret *ast.ReturnStmt) analysis.TextEdit {
	var (
		tokFile    = pass.Fset.File(ret.Pos())
		start, end = tokFile.Offset(ret.Pos()), tokFile.Offset(ret.End())
	)
	end = afterSemicolon(src, end)
	if source.Alone(src, start, end) {
		return analysis.TextEdit{
			Pos: tokFile.Pos(source.Start(src, start)),
			End: tokFile.Pos(source.End(src, end)),
		}
	}
	return analysis.TextEdit{Pos: ret.Pos(), End: tokFile.Pos(end)}
}

// afterSemicolon returns end extended past horizontal space and a semicolon
// that follows the return.
func afterSemicolon(src []byte, end int) int {
	i := end
	for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
		i++
	}
	if i < len(src) && src[i] == ';' {
		return i + 1
	}
	return end
}
