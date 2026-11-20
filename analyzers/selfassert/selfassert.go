// Package selfassert provides an analysis.Analyzer that reports a type
// assertion from a type to itself.
//
// The spec says a type assertion "asserts that x is not nil and that the value
// stored in x is of type T" (https://go.dev/ref/spec#Type_assertions). A non-nil
// interface value always holds a type the interface allows. So asserting a
// value's own type fails only when the value is nil, and the assertion is a
// nil check written the long way.
//
// A two-value assertion yields the value and whether the assertion held, and
// the fix writes the value and the comparison, or the one result a blank
// leaves. It is withheld where the edit would read the value twice without the
// same answer, drop a comment, or cross the width. The one-value form is
// reported without a fix: the assertion is where a nil interface panics, so
// removing it changes what the program does.
//
// The rule is adapted from staticcheck's S1040, which reports "Type assertion
// to current type". go-critic's sloppyTypeAssert reports the same shape.
//
// The README catalogs every check, exemption, and fix.
package selfassert

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "selfassert",
	Doc:  "reports a type assertion from a type to itself",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		parents := astexpr.Parents(file)
		ast.Inspect(file, func(n ast.Node) bool {
			assert, ok := n.(*ast.TypeAssertExpr)
			if !ok || assert.Type == nil {
				return true
			}
			from := pass.TypesInfo.TypeOf(assert.X)
			to := pass.TypesInfo.TypeOf(assert.Type)
			if from == nil || to == nil || !types.Identical(from, to) {
				return true
			}
			diag := analysis.Diagnostic{
				Pos:     assert.Pos(),
				End:     assert.End(),
				Message: "type assertion from a type to itself",
			}
			if suggestion, ok := fix(pass, file, parents, assert); ok {
				diag.SuggestedFixes = []analysis.SuggestedFix{suggestion}
			}
			pass.Report(diag)
			return true
		})
	}
	return nil, nil
}

// fix returns the suggestion that writes what the assertion yields, and
// reports false where the program is left as it is.
func fix(pass *analysis.Pass, file *ast.File, parents map[ast.Node]ast.Node, assert *ast.TypeAssertExpr) (analysis.SuggestedFix, bool) {
	// Two values cannot stand inside a parenthesis, so the edit covers the
	// parentheses as well as the assertion.
	span := ast.Node(assert)
	for {
		paren, ok := parents[span].(*ast.ParenExpr)
		if !ok {
			break
		}
		span = paren
	}
	parent := parents[span]
	bind, ok := binds(parent)
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	// The result is written on one line, so the value has to print on one.
	value, ok := astexpr.OneLine(ast.Unparen(assert.X))
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	var edit analysis.TextEdit
	var message string
	switch {
	case blank(bind.left) && blank(bind.right):
		// Nothing is kept, so there is nothing to write.
		return analysis.SuggestedFix{}, false
	case blank(bind.left) || blank(bind.right):
		assign, ok := parent.(*ast.AssignStmt)
		if !ok {
			return analysis.SuggestedFix{}, false
		}
		edit, message = one(assign, bind, value)
	case !astexpr.Stable(pass, assert.X):
		return analysis.SuggestedFix{}, false
	default:
		edit = analysis.TextEdit{
			Pos:     span.Pos(),
			End:     span.End(),
			NewText: []byte(value + ", " + value + " != nil"),
		}
		message = "Write the value and a nil comparison"
	}
	if astexpr.Commented(file, edit.Pos, edit.End) {
		return analysis.SuggestedFix{}, false
	}
	if !fits(pass, edit.Pos, edit.End, string(edit.NewText)) {
		return analysis.SuggestedFix{}, false
	}
	return analysis.SuggestedFix{
		Message:   message,
		TextEdits: []analysis.TextEdit{edit},
	}, true
}

// binding is the pair of names a statement binds two results to.
type binding struct {
	left, right ast.Expr
}

// binds returns the binding of a parent that reads both results of a two-value
// expression, and reports false for a parent that reads one.
func binds(parent ast.Node) (binding, bool) {
	switch parent := parent.(type) {
	case *ast.AssignStmt:
		if len(parent.Lhs) == 2 && len(parent.Rhs) == 1 {
			return binding{parent.Lhs[0], parent.Lhs[1]}, true
		}
	case *ast.ValueSpec:
		if len(parent.Names) == 2 && len(parent.Values) == 1 {
			return binding{parent.Names[0], parent.Names[1]}, true
		}
	}
	return binding{}, false
}

// one returns the edit that writes the result a blank leaves, and the message
// naming it.
func one(assign *ast.AssignStmt, bind binding, value string) (analysis.TextEdit, string) {
	name, text, message := bind.left, value, "Write the value"
	if blank(bind.left) {
		name, text = bind.right, value+" != nil"
		message = "Write the nil comparison"
	}
	var out strings.Builder
	out.WriteString(astexpr.Text(name))
	out.WriteString(" ")
	out.WriteString(assign.Tok.String())
	out.WriteString(" ")
	out.WriteString(text)
	return analysis.TextEdit{
		Pos:     assign.Pos(),
		End:     assign.End(),
		NewText: []byte(out.String()),
	}, message
}

// blank reports whether e is the blank identifier.
func blank(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// fits reports whether replacing the span with text keeps the line within the
// width. A line already past the width fits: refusing the fix cannot shorten
// it, and linelen reports the line either way.
func fits(pass *analysis.Pass, from, to token.Pos, text string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	// Written positions, which a //line directive does not move.
	start, end := pass.Fset.PositionFor(from, false),
		pass.Fset.PositionFor(to, false)
	if start.Line != end.Line {
		return false
	}
	src, err := pass.ReadFile(start.Filename)
	if err != nil {
		return false
	}
	// A driver that parses other bytes than ReadFile returns puts the
	// position outside the line, so the bounds are checked rather than
	// trusted.
	lines := bytes.Split(src, []byte("\n"))
	if start.Line > len(lines) {
		return false
	}
	line := lines[start.Line-1]
	if start.Column > len(line)+1 || end.Column > len(line)+1 {
		return false
	}
	if !linelen.CheckLine(line) {
		return true
	}
	var rewritten bytes.Buffer
	rewritten.Write(line[:start.Column-1])
	rewritten.WriteString(text)
	rewritten.Write(line[end.Column-1:])
	return linelen.CheckLine(rewritten.Bytes())
}
