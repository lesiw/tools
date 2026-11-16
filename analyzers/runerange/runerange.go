// Package runerange provides an analysis.Analyzer that reports a range over a
// string converted to a rune slice.
//
// The Conversions section of the spec says that converting a string to a slice
// of runes yields the individual Unicode code points of the string
// (https://go.dev/ref/spec#Conversions). The range clause iterates over the
// same code points and yields a rune at each step
// (https://go.dev/ref/spec#For_statements). The conversion allocates a slice
// of every rune in the string, where ranging over the string decodes one rune
// per iteration and allocates nothing.
//
// The two forms differ in their index: ranging over a string produces the byte
// offset of each rune, and ranging over a rune slice produces its position.
// The keys differ when a multi-byte rune is followed by another rune, so a
// loop whose key is not _ is left alone. The fix ranges over the string in
// place of the conversion.
//
// The rule is adapted from staticcheck's SA6003.
//
// The README catalogs every check, exemption, and fix.
package runerange

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "runerange",
	Doc:  "reports a range over a string converted to a rune slice",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			loop, ok := n.(*ast.RangeStmt)
			if !ok || !blank(loop.Key) {
				return true
			}
			conv, src, ok := converted(pass, loop.X)
			if !ok {
				return true
			}
			diag := analysis.Diagnostic{
				Pos:     conv.Pos(),
				End:     conv.End(),
				Message: "string converted to a rune slice; range over the string",
			}
			if edits, ok := unconvert(pass, file, conv, src); ok {
				diag.SuggestedFixes = []analysis.SuggestedFix{{
					Message:   "Range over the string",
					TextEdits: edits,
				}}
			}
			pass.Report(diag)
			return true
		})
	}
	return nil, nil
}

// blank reports whether e is the blank identifier, or absent.
func blank(e ast.Expr) bool {
	if e == nil {
		return true
	}
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// converted returns the conversion e holds and its operand when e converts a
// string to a rune slice.
func converted(pass *analysis.Pass, e ast.Expr) (*ast.CallExpr, ast.Expr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, nil, false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() || !runeSlice(tv.Type) {
		return nil, nil, false
	}
	src := call.Args[0]
	if !stringType(pass.TypesInfo.TypeOf(src)) {
		return nil, nil, false
	}
	return call, src, true
}

// runeSlice reports whether t is a slice whose element is the predeclared
// rune, or a type parameter whose type set holds only such slices. A slice of
// a defined rune type holds values of that type, where a range over a string
// gives rune, so the loop's value would have a different type.
func runeSlice(t types.Type) bool {
	if slice, ok := types.Unalias(t).Underlying().(*types.Slice); ok {
		elem, ok := types.Unalias(slice.Elem()).(*types.Basic)
		return ok && elem.Kind() == types.Rune
	}
	return types.Satisfies(t, runeSliceSet)
}

// runeSliceSet is the type set of ~[]rune, which a type parameter is checked
// against.
var runeSliceSet = types.NewInterfaceType(nil, []types.Type{
	types.NewUnion([]*types.Term{
		types.NewTerm(true, types.NewSlice(types.Typ[types.Rune])),
	}),
}).Complete()

// stringType reports whether a value of type t is a string, including a
// defined string type and a type parameter whose type set holds only strings.
func stringType(t types.Type) bool {
	return t != nil && types.AssignableTo(types.Typ[types.UntypedString], t)
}

// unconvert returns the edits that delete the conversion and its parentheses,
// and reports false when an edit would delete a comment. A space replaces the
// opening text where a name sits immediately before the conversion.
func unconvert(pass *analysis.Pass, file *ast.File, conv *ast.CallExpr, src ast.Expr) ([]analysis.TextEdit, bool) {
	head := analysis.TextEdit{Pos: conv.Pos(), End: src.Pos()}
	if joined(pass, conv.Pos()) {
		head.NewText = []byte(" ")
	}
	edits := []analysis.TextEdit{head, {
		Pos: src.End(),
		End: conv.End(),
	}}
	for _, edit := range edits {
		if commented(file, edit.Pos, edit.End) {
			return nil, false
		}
	}
	return edits, true
}

// joined reports whether the byte before pos is part of a name, so deleting
// text from pos would join that name to the text that follows.
func joined(pass *analysis.Pass, pos token.Pos) bool {
	tokFile := pass.Fset.File(pos)
	if tokFile == nil {
		return false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return false
	}
	at := tokFile.Offset(pos)
	return at > 0 && at <= len(src) && identByte(src[at-1])
}

// identByte reports whether b can appear in an identifier or a number.
func identByte(b byte) bool {
	switch {
	case 'a' <= b && b <= 'z', 'A' <= b && b <= 'Z':
		return true
	case '0' <= b && b <= '9':
		return true
	}
	return b == '_'
}

// commented reports whether a comment sits in the span an edit would replace.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
