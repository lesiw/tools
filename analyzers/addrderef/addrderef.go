// Package addrderef provides an analysis.Analyzer that reports the address of
// a pointer indirection.
//
// The Address operators section of the spec says *x denotes the variable x
// points to, so &*x is x when x and &*x have the same type
// (https://go.dev/ref/spec#Address_operators). The Type identity section gives
// the rule for that comparison (https://go.dev/ref/spec#Type_identity). The
// Address operators section also says &*x panics on a nil x, so the fix is
// offered only where the operand cannot be nil and no comment would be
// removed. A generated file, including cgo output, is left alone. An address
// passed to unsafe.Pointer, possibly through a conversion, is left alone,
// since cgo uses that form to check only the pointed-to object. A defined
// pointer type is left alone too, and so is a type parameter whose core type
// is a pointer: &*x has the unnamed pointer type, and the two expressions have
// different types.
//
// The rule is like staticcheck's SA4001.
package addrderef

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "addrderef",
	Doc:  "reports the address of a pointer indirection",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if generated(pass, file) {
			continue
		}
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			unary, ok := n.(*ast.UnaryExpr)
			if !ok || unary.Op != token.AND {
				return true
			}
			star, ok := ast.Unparen(unary.X).(*ast.StarExpr)
			if !ok {
				return true
			}
			if !sameType(pass, unary, star.X) || unsafePointer(pass, stack) {
				return true
			}
			safe := nonNil(pass, star.X)
			message := "&*x is x"
			if !safe {
				message = "&*x panics on a nil x; x evaluates to nil"
			}
			d := analysis.Diagnostic{
				Pos:     unary.Pos(),
				End:     unary.End(),
				Message: message,
			}
			src, repair := fixText(pass, file, stack, unary, star)
			if repair && !nestedFix(pass, file, stack) {
				d.SuggestedFixes = []analysis.SuggestedFix{{
					Message: "Drop the address and indirection",
					TextEdits: []analysis.TextEdit{{
						Pos:     unary.Pos(),
						End:     unary.End(),
						NewText: src,
					}},
				}}
			}
			pass.Report(d)
			return true
		})
	}
	return nil, nil
}

// sameType reports whether &*x and x have the same type. A defined pointer
// type does not: &*x has the unnamed pointer type.
func sameType(pass *analysis.Pass, unary *ast.UnaryExpr, x ast.Expr) bool {
	a, b := pass.TypesInfo.TypeOf(unary), pass.TypesInfo.TypeOf(x)
	return a != nil && b != nil && types.Identical(a, b)
}

// nonNil reports whether e is an address or a fresh allocation, neither of
// which can be nil. Dropping &* from any other operand can replace the panic
// that &*x raises on a nil x with a nil result.
func nonNil(pass *analysis.Pass, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.UnaryExpr:
		return e.Op == token.AND
	case *ast.CallExpr:
		id, ok := ast.Unparen(e.Fun).(*ast.Ident)
		return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("new")
	}
	return false
}

// generated reports whether file is generated or cgo output. An edit to it
// would change generated text instead of its source.
func generated(pass *analysis.Pass, file *ast.File) bool {
	if ast.IsGenerated(file) {
		return true
	}
	base := filepath.Base(pass.Fset.File(file.Pos()).Name())
	return base == "_cgo_gotypes.go" || base == "_cgo_import.go" ||
		strings.HasSuffix(base, ".cgo1.go") ||
		strings.HasSuffix(base, "-d")
}

// unsafePointer reports whether the node at the top of stack is an argument
// of a call to unsafe.Pointer, possibly through a conversion. The cgo tool
// requires that form to avoid a check on the whole allocation.
func unsafePointer(pass *analysis.Pass, stack []ast.Node) bool {
	for i := len(stack) - 2; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *ast.ParenExpr:
			continue
		case *ast.CallExpr:
			if isUnsafePointer(pass, n) {
				return true
			}
			if !pass.TypesInfo.Types[n.Fun].IsType() {
				return false
			}
		default:
			return false
		}
	}
	return false
}

// isUnsafePointer reports whether call is a conversion to unsafe.Pointer.
func isUnsafePointer(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	typ := pass.TypesInfo.TypeOf(call.Fun)
	return typ != nil && types.Identical(typ, types.Typ[types.UnsafePointer])
}

// repairable reports whether unary can receive a fix without losing a comment,
// changing a nil panic, or dropping an unsafe.Pointer form.
func repairable(pass *analysis.Pass, file *ast.File, stack []ast.Node, unary *ast.UnaryExpr, star *ast.StarExpr) bool {
	if !nonNil(pass, star.X) {
		return false
	}
	if removedComment(file.Comments, unary, star.X) {
		return false
	}
	return !unsafePointer(pass, stack)
}

// fixText returns the replacement for unary and whether it can be fixed.
func fixText(pass *analysis.Pass, file *ast.File, stack []ast.Node, unary *ast.UnaryExpr, star *ast.StarExpr) ([]byte, bool) {
	if !repairable(pass, file, stack, unary, star) {
		return nil, false
	}
	return replacement(pass, unary, star.X)
}

// nestedFix reports whether an enclosing address can receive a fix.
func nestedFix(pass *analysis.Pass, file *ast.File, stack []ast.Node) bool {
	for i := len(stack) - 2; i >= 1; i-- {
		unary, ok := stack[i].(*ast.UnaryExpr)
		if !ok || unary.Op != token.AND {
			continue
		}
		star, ok := ast.Unparen(unary.X).(*ast.StarExpr)
		if !ok || !sameType(pass, unary, star.X) {
			continue
		}
		if _, ok := fixText(pass, file, stack[:i+1], unary, star); ok {
			return true
		}
	}
	return false
}

// removedComment reports whether a comment lies in the text that replacing
// unary with x removes.
func removedComment(groups []*ast.CommentGroup, unary *ast.UnaryExpr, x ast.Expr) bool {
	return commented(groups, unary.Pos(), x.Pos()) ||
		commented(groups, x.End(), unary.End())
}

// replacement returns the text that replaces &*x. It adds a space when the
// byte before the address and the first byte of x would merge into one
// identifier or number.
func replacement(pass *analysis.Pass, unary *ast.UnaryExpr, x ast.Expr) ([]byte, bool) {
	file := pass.Fset.File(x.Pos())
	if file == nil {
		return nil, false
	}
	data, err := pass.ReadFile(file.Name())
	if err != nil {
		return nil, false
	}
	lo, hi := file.Offset(x.Pos()), file.Offset(x.End())
	start := file.Offset(unary.Pos())
	if lo < 0 || hi <= lo || hi > len(data) || start > len(data) {
		return nil, false
	}
	src := bytes.Clone(data[lo:hi])
	if start > 0 && joins(data[start-1], src[0]) {
		src = append([]byte{' '}, src...)
	}
	return src, true
}

// joins reports whether a and b would merge into one identifier or number.
func joins(a, b byte) bool {
	return identByte(a) && identByte(b)
}

// identByte reports whether b is an ASCII letter, digit, or underscore.
func identByte(b byte) bool {
	if b == '_' {
		return true
	}
	if b >= '0' && b <= '9' {
		return true
	}
	if b >= 'A' && b <= 'Z' {
		return true
	}
	return b >= 'a' && b <= 'z'
}

// commented reports whether a comment shares any byte with the half-open span
// from pos to end.
func commented(groups []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
