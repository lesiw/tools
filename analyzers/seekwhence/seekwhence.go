// Package seekwhence provides an analysis.Analyzer that reports a Seek call
// with an io.Seek* constant in the offset argument.
//
// io.Seeker's documentation says Seek "sets the offset for the next Read or
// Write to offset, interpreted according to whence". The first argument is the
// offset and the second is the whence, so a call whose first argument is
// io.SeekStart, io.SeekCurrent, or io.SeekEnd usually has them in the other
// order. A whence constant is also a valid numeric offset: io.SeekStart is 0,
// so an offset of zero can be written as 0.
//
// The rule reads a method call with the signature of io.Seeker.Seek, including
// a concrete type such as *os.File and an interface the author declares with
// that method. A constant declared outside package io is not io.SeekStart,
// whether it is named SeekStart or defined as io.SeekStart, and a method with
// a different signature is not the method io.Seeker declares, so neither is
// reported. A method expression is left alone because its receiver is the
// first argument. A function named Seek, a field of function type named Seek,
// and a stored method value called later are not method calls, so they are
// left alone too. The first argument must be the constant itself, with or
// without parentheses; a conversion or an arithmetic expression around it is
// left alone.
//
// Where the second argument may be an offset, the rule reports without a fix:
// a swap changes the seek target, and writing the first constant as a literal
// would only hide the suspected mistake. Writing 0, 1, or 2 in the first
// argument keeps the numeric offset and clears the report.
//
// Where the second argument contains an io.Seek* constant, the fix writes the
// first constant's value as a literal in the offset argument, which preserves
// behavior. The fix is withheld when a comment inside the first argument would
// be deleted and when the constant in the first argument is dot-imported,
// since replacing it may leave the dot import unused. In particular, no fix is
// offered when the second argument is a non-literal constant with the value 0,
// 1, or 2 that contains no io.Seek* constant, since that value may be the
// offset. The diagnostic names the value.
//
// The rule is adapted from staticcheck's SA1013.
package seekwhence

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "seekwhence",
	Doc:  "reports a Seek call with an io.Seek* constant in the offset argument",
	Run:  run,
}

// seeker is the signature of io.Seeker.Seek, without a receiver.
var seeker = types.NewSignatureType(
	nil, nil, nil,
	types.NewTuple(
		types.NewParam(token.NoPos, nil, "", types.Typ[types.Int64]),
		types.NewParam(token.NoPos, nil, "", types.Typ[types.Int]),
	),
	types.NewTuple(
		types.NewParam(token.NoPos, nil, "", types.Typ[types.Int64]),
		types.NewParam(
			token.NoPos, nil, "",
			types.Universe.Lookup("error").Type(),
		),
	),
	false,
)

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

func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) {
	if len(call.Args) != 2 {
		return
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Seek" || !isSeeker(pass, sel) {
		return
	}
	name, ok := whence(pass, call.Args[0])
	if !ok {
		return
	}
	d := analysis.Diagnostic{
		Pos: call.Args[0].Pos(),
		End: call.Args[0].End(),
		Message: "the first argument of Seek is the offset; " +
			name + " belongs in the second",
	}
	if hasWhence(pass, call.Args[1]) {
		d.Message = "the first argument of Seek is the offset; both arguments contain an io.Seek constant"
	} else if value, ok := constantWhence(pass, call.Args[1]); ok {
		d.Message = "the first argument of Seek is the offset; the second argument has the constant value " + value
	}
	if fix := fix(pass, file, call); fix != nil {
		d.SuggestedFixes = []analysis.SuggestedFix{*fix}
	}
	pass.Report(d)
}

// isSeeker reports whether sel is a method value with the signature of
// io.Seeker.Seek. A method expression has the receiver among its arguments and
// is not one.
func isSeeker(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	selection, ok := pass.TypesInfo.Selections[sel]
	return ok && selection.Kind() == types.MethodVal &&
		types.Identical(selection.Type(), seeker)
}

// whence returns the name of the io.Seek* constant e is, and reports whether e
// is one.
func whence(pass *analysis.Pass, e ast.Expr) (string, bool) {
	var id *ast.Ident
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	default:
		return "", false
	}
	c, ok := pass.TypesInfo.ObjectOf(id).(*types.Const)
	if !ok || c.Pkg() == nil || c.Pkg().Path() != "io" {
		return "", false
	}
	switch c.Name() {
	case "SeekStart", "SeekCurrent", "SeekEnd":
		return c.Name(), true
	}
	return "", false
}

// hasWhence reports whether e contains an io.Seek* constant.
func hasWhence(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if found {
			return false
		}
		if expr, ok := n.(ast.Expr); ok {
			if _, ok := whence(pass, expr); ok {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// constantWhence returns the value of e when e is a non-literal constant
// expression with the value of a whence. Such an expression can stand for the
// offset or for the whence, so the message does not say which one it is.
func constantWhence(pass *analysis.Pass, e ast.Expr) (string, bool) {
	e = ast.Unparen(e)
	if _, ok := e.(*ast.BasicLit); ok {
		return "", false
	}
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil {
		return "", false
	}
	i := constant.ToInt(tv.Value)
	if i.Kind() != constant.Int {
		return "", false
	}
	v, ok := constant.Int64Val(i)
	if !ok {
		return "", false
	}
	switch v {
	case 0, 1, 2:
		return strconv.FormatInt(v, 10), true
	}
	return "", false
}

// fix returns the edit that writes the offset as a literal, or nil where the
// second argument may be an offset, the first argument is dot-imported, or a
// comment in the first argument would be deleted.
func fix(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) *analysis.SuggestedFix {
	if !hasWhence(pass, call.Args[1]) {
		return nil
	}
	if _, ok := ast.Unparen(call.Args[0]).(*ast.Ident); ok {
		return nil
	}
	if commented(file.Comments, call.Args[0].Pos(), call.Args[0].End()) {
		return nil
	}
	value, ok := constantValue(pass, call.Args[0])
	if !ok {
		return nil
	}
	return &analysis.SuggestedFix{
		Message: "Write the offset as a literal",
		TextEdits: []analysis.TextEdit{{
			Pos:     call.Args[0].Pos(),
			End:     call.Args[0].End(),
			NewText: []byte(value),
		}},
	}
}

// constantValue returns the value of e as source text.
func constantValue(pass *analysis.Pass, e ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[ast.Unparen(e)]
	if !ok || tv.Value == nil {
		return "", false
	}
	return tv.Value.ExactString(), true
}

// commented reports whether a comment overlaps the span from pos to end.
func commented(groups []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
