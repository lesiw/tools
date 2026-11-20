// Package taggedswitch provides an analysis.Analyzer that reports an untagged
// switch that compares one expression with a series of values.
//
// The spec's expression switches section
// (https://go.dev/ref/spec#Expression_switches) says an expression switch
// compares its expression with each case expression for equality. An untagged
// switch that compares the same expression in every case repeats it once per
// comparison, and the tag names it once.
//
// The fix writes the tag and replaces each comparison with the value it tests
// against. It is withheld where the edit would delete a comment, repeat a
// constant, take a line past the width, or write the header across lines, and
// the switch is then reported without it. The README catalogs every check,
// exemption, and fix.
//
// Adapted from staticcheck's QF1002, which reports an untagged switch that
// compares a single variable against a series of values.
package taggedswitch

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "taggedswitch",
	Doc:  "reports an untagged switch that compares one expression with a series of values",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if stmt, ok := n.(*ast.SwitchStmt); ok {
				check(pass, file, stmt)
			}
			return true
		})
	}
	return nil, nil
}

func check(pass *analysis.Pass, file *ast.File, stmt *ast.SwitchStmt) {
	clauses, compared, tag := collect(pass, stmt)
	if tag == nil || !astexpr.Stable(pass, tag) || !switched(pass, tag) {
		return
	}
	values, fixable, ok := valueTexts(pass, compared)
	if !ok {
		return
	}
	expr := ast.Unparen(tag)
	text, oneLine := astexpr.OneLine(expr)
	if astexpr.Composite(expr) {
		// A switch header cannot begin with a composite literal, whose
		// brace the parser reads as the body.
		text = "(" + text + ")"
	}
	diagnostic := analysis.Diagnostic{
		Pos:     stmt.Switch,
		End:     stmt.Body.Lbrace,
		Message: "switch without a tag",
	}
	fix := fixable && oneLine && !commented(file, clauses) &&
		fits(pass, stmt, clauses, values, text)
	if fix {
		diagnostic.Message += "; write switch " + header(stmt, text)
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Add the tag",
			TextEdits: edits(stmt, clauses, values, text),
		}}
	}
	pass.Report(diagnostic)
}

// collect returns each case clause's equality comparisons and the expression
// they all compare. A switch whose clauses compare anything else, or more than
// one expression, returns no tag.
func collect(pass *analysis.Pass, stmt *ast.SwitchStmt) (clauses []*ast.CaseClause, compared [][]*ast.BinaryExpr, tag ast.Expr) {
	if stmt.Tag != nil {
		return nil, nil, nil
	}
	for _, node := range stmt.Body.List {
		var (
			clause = node.(*ast.CaseClause)
			pairs  []*ast.BinaryExpr
		)
		for _, expr := range clause.List {
			if !comparisons(expr, &pairs) {
				return nil, nil, nil
			}
		}
		for _, pair := range pairs {
			if tag == nil {
				tag = pair.X
			} else if !astexpr.SamePure(pass, tag, pair.X) {
				return nil, nil, nil
			}
		}
		clauses = append(clauses, clause)
		compared = append(compared, pairs)
	}
	return clauses, compared, tag
}

// comparisons appends the equality comparisons expr holds to pairs, and
// reports whether expr is one of them or a || of them.
func comparisons(expr ast.Expr, pairs *[]*ast.BinaryExpr) bool {
	binary, ok := ast.Unparen(expr).(*ast.BinaryExpr)
	if !ok {
		return false
	}
	switch binary.Op {
	case token.LOR:
		return comparisons(binary.X, pairs) && comparisons(binary.Y, pairs)
	case token.EQL:
		*pairs = append(*pairs, binary)
		return true
	}
	return false
}

// valueTexts returns the text of every value the switch compares. It reports
// whether a tagged switch accepts the values as written, and whether reading
// every value twice gives the same answer.
func valueTexts(pass *analysis.Pass, compared [][]*ast.BinaryExpr) (values [][]string, fixable, ok bool) {
	fixable = true
	values = make([][]string, len(compared))
	var consts []caseValue
	for i, pairs := range compared {
		for _, pair := range pairs {
			expr := ast.Unparen(pair.Y)
			if astexpr.Effectful(pass, expr) {
				return nil, false, false
			}
			values[i] = append(values[i], astexpr.Text(expr))
			tv := pass.TypesInfo.Types[expr]
			if tv.Value == nil {
				continue
			}
			// A switch rejects two case values that are the same constant
			// of the same type, which is the type it converted each to.
			v := caseValue{tv.Type, tv.Value}
			if repeats(consts, v) {
				fixable = false
			}
			consts = append(consts, v)
		}
	}
	return values, fixable, true
}

// switched reports whether this rule writes e as a tag. A constant is not
// written: it would be the tag, and the cases would name the expression. The
// untyped value that is no constant is nil, and a switch expression cannot be
// nil.
func switched(pass *analysis.Pass, e ast.Expr) bool {
	return pass.TypesInfo.Types[e].Value == nil &&
		!untyped(pass.TypesInfo.TypeOf(e))
}

// untyped reports whether t is an untyped type.
func untyped(t types.Type) bool {
	if t == nil {
		return true
	}
	basic, ok := types.Unalias(t).(*types.Basic)
	return ok && basic.Info()&types.IsUntyped != 0
}

// caseValue is one constant a switch compares, with the type the switch
// converted it to.
type caseValue struct {
	typ types.Type
	val constant.Value
}

// repeats reports whether v is a constant the switch already holds.
func repeats(seen []caseValue, v caseValue) bool {
	for _, other := range seen {
		if !types.Identical(other.typ, v.typ) || !sameKind(other.val, v.val) {
			continue
		}
		if constant.Compare(other.val, token.EQL, v.val) {
			return true
		}
	}
	return false
}

// sameKind reports whether two constants are of kinds constant.Compare takes
// together: one kind, or two of the three numeric ones.
func sameKind(a, b constant.Value) bool {
	if a.Kind() == b.Kind() {
		return true
	}
	return numeric(a) && numeric(b)
}

// numeric reports whether v is a number.
func numeric(v constant.Value) bool {
	switch v.Kind() {
	case constant.Int, constant.Float, constant.Complex:
		return true
	}
	return false
}

// commented reports whether a comment lies between a case's first expression
// and its colon, where the rewrite would delete it.
func commented(file *ast.File, clauses []*ast.CaseClause) bool {
	for _, clause := range clauses {
		if len(clause.List) == 0 {
			continue
		}
		if astexpr.Commented(file, clause.List[0].Pos(), clause.Colon) {
			return true
		}
	}
	return false
}

// header returns the switch header a tagged switch would write.
func header(stmt *ast.SwitchStmt, tag string) string {
	if stmt.Init == nil {
		return tag
	}
	return astexpr.Text(stmt.Init) + "; " + tag
}

// fits reports whether the fix keeps every line it rewrites within the width.
func fits(pass *analysis.Pass, stmt *ast.SwitchStmt, clauses []*ast.CaseClause, values [][]string, tag string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	tokFile := pass.Fset.File(stmt.Switch)
	if tokFile == nil {
		return false
	}
	src, err := pass.ReadFile(tokFile.Name())
	// The offsets index the bytes the driver parsed, so a file that cannot be
	// read, or that changed size, is not used.
	if err != nil || tokFile.Size() != len(src) {
		return false
	}
	head := tokFile.Offset(stmt.Switch)
	tail := tokFile.Offset(stmt.Body.Lbrace)
	if !fitsLine(src, head, tail, "switch "+header(stmt, tag)+" ") {
		return false
	}
	for i, clause := range clauses {
		if len(clause.List) == 0 {
			continue
		}
		lo, hi := tokFile.Offset(clause.Case), tokFile.Offset(clause.Colon)
		if !fitsLine(src, lo, hi, strings.Join(values[i], ", ")) {
			return false
		}
	}
	return true
}

// fitsLine reports whether replacing src[lo:hi] with text leaves the line it
// lands on within the width. A clause broken across lines is measured as the
// one line it becomes.
func fitsLine(src []byte, lo, hi int, text string) bool {
	var line []byte
	line = append(line, src[source.Start(src, lo):lo]...)
	line = append(line, text...)
	end := source.End(src, hi)
	line = append(line, bytes.TrimSuffix(src[hi:end], []byte("\n"))...)
	return linelen.CheckLine(line)
}

// edits writes the tag and replaces each comparison with the values it
// compared.
func edits(stmt *ast.SwitchStmt, clauses []*ast.CaseClause, values [][]string, tag string) []analysis.TextEdit {
	out := []analysis.TextEdit{{
		Pos:     stmt.Body.Lbrace,
		NewText: []byte(" " + tag),
	}}
	for i, clause := range clauses {
		if len(clause.List) == 0 {
			continue
		}
		out = append(out, analysis.TextEdit{
			Pos:     clause.List[0].Pos(),
			End:     clause.Colon,
			NewText: []byte(strings.Join(values[i], ", ")),
		})
	}
	return out
}
