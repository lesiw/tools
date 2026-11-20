// Package writestring provides an analysis.Analyzer that reports a call to
// WriteString whose argument concatenates strings.
//
// The spec says string addition "creates a new string by concatenating the
// operands" (https://go.dev/ref/spec#String_concatenation), and WriteString
// copies that string into the writer, so the bytes are copied twice. A call
// per operand copies each part once.
//
// The call is reported when the writer is a bufio.Writer, a bytes.Buffer, a
// hash/maphash.Hash, or a strings.Builder, and the call stands alone in a
// statement list, with a label or without one. The receiver must be stable: a
// call in it must give the same value twice, and a receive or the address of a
// literal such as &T{} makes it unstable. No operand after the first may name
// anything the receiver names. A call on an *os.File or through io.WriteString
// is left alone, and so is a subexpression the compiler folds. The fix is
// withheld where a comment sits inside the call. The README catalogs every
// check, exemption, and fix.
//
// Vendored from gopls' writestring (gopls/internal/analysis/writestring,
// golang.org/x/tools/gopls v0.24.0), which is internal and cannot be imported.
// It is upstream's, except that this reports a parenthesized argument and
// leaves alone a call in a header, a receiver the split would evaluate to a
// new value, and an operand after the first that names what the receiver
// names.
package writestring

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "writestring",
	Doc:  "reports a WriteString call whose argument concatenates strings",
	Run:  run,
}

// writers maps a package path to the type whose WriteString the rule covers.
var writers = map[string]string{
	"bufio":        "Writer",
	"bytes":        "Buffer",
	"hash/maphash": "Hash",
	"strings":      "Builder",
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BlockStmt:
				checkList(pass, file, n.List)
			case *ast.CaseClause:
				checkList(pass, file, n.Body)
			case *ast.CommClause:
				checkList(pass, file, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// checkList reports each call in list that concatenates strings.
func checkList(pass *analysis.Pass, file *ast.File, list []ast.Stmt) {
	for _, stmt := range list {
		expr, ok := unlabel(stmt).(*ast.ExprStmt)
		if !ok {
			continue
		}
		if call, ok := expr.X.(*ast.CallExpr); ok {
			check(pass, file, call)
		}
	}
}

// unlabel returns the statement a label holds.
func unlabel(stmt ast.Stmt) ast.Stmt {
	for {
		label, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			return stmt
		}
		stmt = label.Stmt
	}
}

// check reports call when its argument concatenates two or more strings.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return
	}
	if len(call.Args) != 1 {
		return
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || !writesStrings(fn) {
		return
	}
	if !astexpr.Stable(pass, sel.X) {
		return
	}
	parts := operands(pass, call.Args[0])
	if len(parts) < 2 || readsReceiver(pass, sel.X, parts) {
		return
	}
	report := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "concatenation copied into the writer; write each operand",
	}
	if fix := replacement(file, call, parts); fix != nil {
		report.SuggestedFixes = []analysis.SuggestedFix{*fix}
	}
	pass.Report(report)
}

// replacement returns the fix that writes each operand with its own call, and
// nothing when a comment sits inside the call, which the fix would delete.
func replacement(file *ast.File, call *ast.CallExpr, parts []ast.Expr) *analysis.SuggestedFix {
	if commented(file, call.Pos(), call.End()) {
		return nil
	}
	var (
		fun   = astexpr.Text(call.Fun)
		calls = make([]string, len(parts))
	)
	for i, part := range parts {
		calls[i] = fun + "(" + astexpr.Text(part) + ")"
	}
	return &analysis.SuggestedFix{
		Message: "Split into one call per operand",
		TextEdits: []analysis.TextEdit{{
			Pos:     call.Pos(),
			End:     call.End(),
			NewText: []byte(strings.Join(calls, "\n")),
		}},
	}
}

// writesStrings reports whether fn is WriteString on one of the writers.
func writesStrings(fn *types.Func) bool {
	if fn.Name() != "WriteString" {
		return false
	}
	recv := fn.Signature().Recv()
	if recv == nil {
		return false
	}
	t := recv.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && writers[obj.Pkg().Path()] == obj.Name()
}

// operands returns the strings expr joins, keeping a folded subexpression
// whole.
func operands(pass *analysis.Pass, expr ast.Expr) []ast.Expr {
	if pass.TypesInfo.Types[expr].Value != nil {
		return []ast.Expr{expr}
	}
	bin, ok := ast.Unparen(expr).(*ast.BinaryExpr)
	if !ok {
		return []ast.Expr{expr}
	}
	return append(operands(pass, bin.X), operands(pass, bin.Y)...)
}

// readsReceiver reports whether an operand after the first names something the
// receiver names, which the call before it has written to. A value is named by
// its object, so x and &x name the same object.
func readsReceiver(pass *analysis.Pass, receiver ast.Expr, parts []ast.Expr) bool {
	var names []types.Object
	for n := range ast.Preorder(receiver) {
		if obj := object(pass, n); obj != nil {
			names = append(names, obj)
		}
	}
	for _, part := range parts[1:] {
		for n := range ast.Preorder(part) {
			obj := object(pass, n)
			if obj != nil && slices.Contains(names, obj) {
				return true
			}
		}
	}
	return false
}

// object returns the object n names, if it names one.
func object(pass *analysis.Pass, n ast.Node) types.Object {
	switch n := n.(type) {
	case *ast.Ident:
		return pass.TypesInfo.ObjectOf(n)
	case *ast.SelectorExpr:
		return pass.TypesInfo.ObjectOf(n.Sel)
	}
	return nil
}

// commented reports whether a comment sits between lo and hi, which the fix
// would delete.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if lo <= group.Pos() && group.End() <= hi {
			return true
		}
	}
	return false
}
