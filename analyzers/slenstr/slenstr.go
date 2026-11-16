// Package slenstr provides an analysis.Analyzer that reports a string
// converted to a byte slice only to take its length.
//
// A conversion from a string to a byte slice yields a slice whose elements
// are the bytes of the string (https://go.dev/ref/spec#Conversions), and len
// reports the length of a string in bytes
// (https://go.dev/ref/spec#Length_and_capacity). Both forms count the same
// bytes, and len(s) does not build the slice.
//
// A conversion of or to a type parameter is left alone, as is a call to a len
// declared in an enclosing scope. The target can be any slice whose elements
// are bytes, including a defined byte slice and a slice of a defined byte
// type.
//
// The rule is inspired by go-critic's stringXbytes, which reports the reverse
// length check, len(string(b)).
//
// The fix drops the conversion. It is withheld when the operand is constant,
// since removing the conversion can make the call constant, and when an edit
// would delete a comment.
package slenstr

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "slenstr",
	Doc:  "reports a string converted to a byte slice only to take its length",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isLen(pass, call) {
				return true
			}
			arg := call.Args[0]
			conv, target, ok := converted(pass, arg)
			if !ok || !byteSlice(target) {
				return true
			}
			src := conv.Args[0]
			if !isString(pass.TypesInfo.TypeOf(src)) {
				return true
			}
			pass.Report(diagnose(pass, file, arg, src))
			return true
		})
	}
	return nil, nil
}

// isLen reports whether call is a one-argument call to the predeclared len.
func isLen(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("len")
}

// converted returns the conversion e, its target type, and whether e is a
// one-argument conversion.
func converted(pass *analysis.Pass, e ast.Expr) (*ast.CallExpr, types.Type, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, nil, false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return nil, nil, false
	}
	return call, tv.Type, true
}

// byteSlice reports whether t is a slice whose elements are bytes.
func byteSlice(t types.Type) bool {
	if t == nil {
		return false
	}
	slice, ok := types.Unalias(t).Underlying().(*types.Slice)
	if !ok {
		return false
	}
	basic, ok := slice.Elem().Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Byte
}

// isString reports whether t is a string or has a string underlying type.
func isString(t types.Type) bool {
	if t == nil {
		return false
	}
	basic, ok := types.Unalias(t).Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

// diagnose returns the diagnostic for arg, with the conversion dropped as its
// fix where the operand is not constant and no edit would delete a comment.
func diagnose(pass *analysis.Pass, file *ast.File, arg, src ast.Expr) analysis.Diagnostic {
	diag := analysis.Diagnostic{
		Pos:     arg.Pos(),
		End:     arg.End(),
		Message: "conversion only to take its length; len counts the bytes directly",
	}
	if tv, ok := pass.TypesInfo.Types[src]; ok && tv.Value != nil {
		return diag
	}
	edits := []analysis.TextEdit{
		{Pos: arg.Pos(), End: src.Pos()},
		{Pos: src.End(), End: arg.End()},
	}
	for _, edit := range edits {
		if commented(file.Comments, edit.Pos, edit.End) {
			return diag
		}
	}
	diag.SuggestedFixes = []analysis.SuggestedFix{{
		Message:   "Drop the conversion",
		TextEdits: edits,
	}}
	return diag
}

// commented reports whether a comment sits in the span an edit would delete.
func commented(comments []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
