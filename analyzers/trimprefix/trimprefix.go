// Package trimprefix provides an analysis.Analyzer that reports a HasPrefix
// guard around a TrimPrefix of the same prefix assigned back to the same
// string or byte slice.
//
// The strings.TrimPrefix doc says it returns s unchanged when s does not start
// with prefix, and the bytes.TrimPrefix doc says the same. Both are documented
// at https://pkg.go.dev/strings#TrimPrefix and
// https://pkg.go.dev/bytes#TrimPrefix. The guard adds nothing to the trimmed
// value.
//
// The fix removes the guard. An else if keeps its braces, and no fix is
// offered when a comment lies between the if and its opening brace.
//
// The rule is adapted from staticcheck's S1017, which reports a HasPrefix
// followed by a manual slice and rewrites it to TrimPrefix. This rule reports
// a HasPrefix guard whose test the TrimPrefix call already covers.
//
// The rule applies to files before Go 1.20. At Go 1.20 and later, modernize's
// stringscutprefix reports the pair and rewrites it to CutPrefix in the
// matching package. A file with no version information is left alone. The
// -trimprefix.modern flag reports the pair anyway.
package trimprefix

import (
	"go/ast"
	"go/token"
	"go/types"
	"go/version"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "trimprefix",
	Doc:  "reports a redundant HasPrefix guard around TrimPrefix before Go 1.20",
	Run:  run,
}

// modern enables reporting at Go 1.20 and later, where stringscutprefix
// reports the pair.
var modern bool

func init() {
	Analyzer.Flags.BoolVar(
		&modern, "modern", false,
		"report at Go 1.20 and later",
	)
}

// trims maps a HasPrefix function to the TrimPrefix that tests the same
// prefix.
var trims = map[string]string{
	"bytes.HasPrefix":   "bytes.TrimPrefix",
	"strings.HasPrefix": "strings.TrimPrefix",
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if !beforeCutPrefix(pass, file) {
			continue
		}
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			src = nil
		}
		chained := elseIfs(file)
		ast.Inspect(file, func(n ast.Node) bool {
			if stmt, ok := n.(*ast.IfStmt); ok {
				check(pass, file, src, stmt, chained[stmt])
			}
			return true
		})
	}
	return nil, nil
}

// beforeCutPrefix reports whether file is before Go 1.20. A file with no
// version information is left alone, as modernize does.
func beforeCutPrefix(pass *analysis.Pass, file *ast.File) bool {
	if modern {
		return true
	}
	v, ok := pass.TypesInfo.FileVersions[file]
	return ok && v != "" && version.Compare(v, "go1.20") < 0
}

// elseIfs reports the ifs that an else reaches.
func elseIfs(file *ast.File) map[*ast.IfStmt]bool {
	out := make(map[*ast.IfStmt]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		if stmt, ok := n.(*ast.IfStmt); ok {
			if inner, ok := stmt.Else.(*ast.IfStmt); ok {
				out[inner] = true
			}
		}
		return true
	})
	return out
}

// check reports stmt when its body only trims a prefix the condition already
// matched. chained reports that an else reaches stmt.
func check(pass *analysis.Pass, file *ast.File, src []byte, stmt *ast.IfStmt, chained bool) {
	if stmt.Init != nil || stmt.Else != nil || len(stmt.Body.List) != 1 {
		return
	}
	cond, ok := ast.Unparen(stmt.Cond).(*ast.CallExpr)
	if !ok || len(cond.Args) != 2 {
		return
	}
	trim, ok := trims[callName(pass, cond)]
	if !ok {
		return
	}
	if !trimsBody(pass, stmt.Body.List[0], trim, cond.Args[0], cond.Args[1]) {
		return
	}
	diag := analysis.Diagnostic{
		Pos:     stmt.Pos(),
		End:     stmt.End(),
		Message: "redundant HasPrefix guard around TrimPrefix",
	}
	if edits, ok := fixEdits(pass, file, src, stmt, chained); ok {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Drop the HasPrefix guard",
			TextEdits: edits,
		}}
	}
	pass.Report(diag)
}

// trimsBody reports whether stmt assigns a trim of s and prefix back to s.
func trimsBody(pass *analysis.Pass, stmt ast.Stmt, trim string, s, prefix ast.Expr) bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	if !astexpr.Same(pass, assign.Lhs[0], s) {
		return false
	}
	if writesThroughMap(pass, assign.Lhs[0]) {
		return false
	}
	call, ok := ast.Unparen(assign.Rhs[0]).(*ast.CallExpr)
	return ok && trimsSame(pass, call, trim, s, prefix)
}

// writesThroughMap reports whether writing e creates a map key. Only the
// index being written can create one; a map read below it, as in m[k][0],
// happens while reading the target and panics on a missing key either way.
func writesThroughMap(pass *analysis.Pass, e ast.Expr) bool {
	index, ok := ast.Unparen(e).(*ast.IndexExpr)
	if !ok {
		return false
	}
	return mayBeMap(pass.TypesInfo.TypeOf(index.X))
}

// mayBeMap reports whether t is not known to be a writable index target.
func mayBeMap(t types.Type) bool {
	return t == nil || !astexpr.IsWritableIndex(t)
}

// trimsSame reports whether call is a call to trim over s and prefix.
func trimsSame(pass *analysis.Pass, call *ast.CallExpr, trim string, s, prefix ast.Expr) bool {
	if len(call.Args) != 2 || callName(pass, call) != trim {
		return false
	}
	return astexpr.Same(pass, call.Args[0], s) &&
		astexpr.Same(pass, call.Args[1], prefix)
}

// callName returns the package-qualified name of the function a call names,
// or the empty string when the call does not name a package function.
func callName(pass *analysis.Pass, call *ast.CallExpr) string {
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		fn, ok := pass.TypesInfo.Uses[fun].(*types.Func)
		if !ok || fn.Pkg() == nil {
			return ""
		}
		return fn.Pkg().Path() + "." + fn.Name()
	case *ast.SelectorExpr:
		fn, ok := pass.TypesInfo.Uses[fun.Sel].(*types.Func)
		if !ok || fn.Pkg() == nil {
			return ""
		}
		ident, ok := ast.Unparen(fun.X).(*ast.Ident)
		if !ok {
			return ""
		}
		pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
		if !ok || pkg.Imported().Path() != fn.Pkg().Path() {
			return ""
		}
		return fn.Pkg().Path() + "." + fn.Name()
	}
	return ""
}

// fixEdits returns the edits that remove the guard, leaving the assignment.
// An else keeps its braces, since an else must be followed by a block or an
// if. The edits are withheld when the header holds a comment they would
// delete.
func fixEdits(pass *analysis.Pass, file *ast.File, src []byte, stmt *ast.IfStmt, chained bool) ([]analysis.TextEdit, bool) {
	if src == nil {
		return nil, false
	}
	if chained {
		header, ok := elseHeaderEdit(file, stmt)
		if !ok {
			return nil, false
		}
		return []analysis.TextEdit{header}, true
	}
	header, ok := headerEdit(pass, src, file, stmt)
	if !ok {
		return nil, false
	}
	return []analysis.TextEdit{
		header,
		braceEdit(pass, src, stmt.Body.Rbrace),
	}, true
}

// elseHeaderEdit returns the edit that removes the if and its condition from
// an else if, leaving the else block.
func elseHeaderEdit(file *ast.File, stmt *ast.IfStmt) (analysis.TextEdit, bool) {
	lo, hi := stmt.Pos(), stmt.Body.Lbrace
	if hasComment(file, lo, hi) {
		return analysis.TextEdit{}, false
	}
	return analysis.TextEdit{Pos: lo, End: hi}, true
}

// headerEdit returns the edit that deletes the if header. A header alone on
// its lines leaves no blank line behind.
func headerEdit(pass *analysis.Pass, src []byte, file *ast.File, stmt *ast.IfStmt) (analysis.TextEdit, bool) {
	lo, hi := stmt.Pos(), stmt.Body.Lbrace+1
	if hasComment(file, lo, hi) {
		return analysis.TextEdit{}, false
	}
	tokFile := pass.Fset.File(stmt.Pos())
	start, end := tokFile.Offset(lo), tokFile.Offset(hi)
	if source.Alone(src, start, end) {
		return analysis.TextEdit{
			Pos: tokFile.Pos(source.Start(src, start)),
			End: tokFile.Pos(source.End(src, end)),
		}, true
	}
	return analysis.TextEdit{Pos: lo, End: hi}, true
}

// hasComment reports whether a comment has any part in the half-open range lo
// to hi.
func hasComment(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}

// braceEdit returns the edit that deletes a closing brace and its line when
// the brace is alone on it.
func braceEdit(pass *analysis.Pass, src []byte, brace token.Pos) analysis.TextEdit {
	var (
		tokFile = pass.Fset.File(brace)
		at      = tokFile.Offset(brace)
	)
	if source.Alone(src, at, at+1) {
		return analysis.TextEdit{
			Pos: tokFile.Pos(source.Start(src, at)),
			End: tokFile.Pos(source.End(src, at)),
		}
	}
	return analysis.TextEdit{Pos: brace, End: brace + 1}
}
