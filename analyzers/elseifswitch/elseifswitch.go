// Package elseifswitch provides an analysis.Analyzer that reports an if-else
// chain that compares one expression with == on every branch.
//
// Effective Go's section on switch gives the reason: "It's therefore
// possible—and idiomatic—to write an if-else-if-else chain as a switch"
// (https://go.dev/doc/effective_go#switch). Each branch of the chain becomes
// one case clause, and a branch whose condition joins comparisons with ||
// becomes a clause that lists the values it compares, since the same section
// notes that cases "can be presented in comma-separated lists".
//
// A switch reads its expression once, where a chain may read it several times,
// so the rule requires the expression every comparison reads to give the same
// value on a second read.
//
// A chain needs two ifs: a single if and an else is a choice between two
// things, however many values its condition lists.
//
// The rule reads a chain from its first if, so a chain that begins at an else
// if is left alone. One that is alone in an else block is left to elseif,
// which folds the block into else if.
//
// Safe fix: the if becomes switch e {, each branch becomes a case clause, and
// an else block becomes default. A fix is withheld where the chain cannot be
// written as one switch, or where the edit would lose a comment. The README
// catalogs every check, exemption, and fix.
//
// The rule is adapted from staticcheck's QF1003, which reports an if-else-if
// chain that compares one value with ==.
package elseifswitch

import (
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
	Name: "elseifswitch",
	Doc:  "reports an if-else chain that compares one expression with ==",
	Run:  run,
}

// branch is one if of a chain, with the values its condition compares the tag
// against.
type branch struct {
	stmt   *ast.IfStmt
	values []ast.Expr
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		parents := astexpr.Parents(file)
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			if startsChain(parents, stmt) {
				report(pass, file, stmt)
			}
			return true
		})
	}
	return nil, nil
}

// report reports stmt when it starts a chain that a switch writes.
func report(pass *analysis.Pass, file *ast.File, stmt *ast.IfStmt) {
	branches, tag, ok := chain(pass, stmt)
	if !ok {
		return
	}
	tagName := astexpr.Text(ast.Unparen(tag))
	diagnostic := analysis.Diagnostic{
		Pos:     stmt.If,
		End:     stmt.Cond.End(),
		Message: "if-else chain on " + tagName + "; write a switch",
	}
	if edits, ok := rewrite(pass, file, branches, tag); ok {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Replace the chain with a switch",
			TextEdits: edits,
		}}
	}
	pass.Report(diagnostic)
}

// startsChain reports whether stmt starts a chain this rule can write. An else
// if starts none, and neither does an if that is alone in an else block, which
// elseif folds into else if.
func startsChain(parents map[ast.Node]ast.Node, stmt *ast.IfStmt) bool {
	if _, ok := parents[stmt].(*ast.IfStmt); ok {
		return false
	}
	block, ok := parents[stmt].(*ast.BlockStmt)
	if !ok || len(block.List) != 1 {
		return true
	}
	outer, ok := parents[block].(*ast.IfStmt)
	return !ok || outer.Else != block
}

// chain returns the chain that stmt starts, and the expression every branch
// compares, and reports false when a switch cannot write it.
func chain(pass *analysis.Pass, stmt *ast.IfStmt) ([]branch, ast.Expr, bool) {
	var (
		branches []branch
		tag      ast.Expr
		seen     = make(map[string]struct{})
		current  = stmt
	)
	for {
		if current.Init != nil || captures(current) {
			return nil, nil, false
		}
		values, x, ok := comparisons(pass, current.Cond)
		if !ok {
			return nil, nil, false
		}
		if tag == nil {
			tag = x
		} else if !astexpr.SamePure(pass, tag, x) {
			return nil, nil, false
		}
		for _, value := range values {
			key := caseKey(pass, value)
			if _, ok := seen[key]; ok {
				return nil, nil, false
			}
			seen[key] = struct{}{}
		}
		branches = append(branches, branch{stmt: current, values: values})
		next, ok := current.Else.(*ast.IfStmt)
		if !ok {
			break
		}
		current = next
	}
	if len(branches) < 2 {
		return nil, nil, false
	}
	return branches, tag, true
}

// caseKey returns what a case expression names, so that two branches holding
// one value are recognized. A constant is keyed by its type as well as its
// value, since 1 and 1.0 are one case on an int and two on an interface.
func caseKey(pass *analysis.Pass, value ast.Expr) string {
	tv, ok := pass.TypesInfo.Types[value]
	if !ok || tv.Value == nil {
		return astexpr.Text(ast.Unparen(value))
	}
	return tv.Type.String() + ": " + tv.Value.ExactString()
}

// comparisons returns the values cond compares with, and the expression it
// compares them against. The condition is one == that reads the same on a
// second read, or a || of conditions of that shape.
func comparisons(pass *analysis.Pass, cond ast.Expr) ([]ast.Expr, ast.Expr, bool) {
	e, ok := ast.Unparen(cond).(*ast.BinaryExpr)
	if !ok {
		return nil, nil, false
	}
	switch e.Op {
	case token.EQL:
		if !astexpr.Stable(pass, e.X) || !astexpr.Stable(pass, e.Y) {
			return nil, nil, false
		}
		if untyped(pass, e.X) {
			return nil, nil, false
		}
		return []ast.Expr{e.Y}, e.X, true
	case token.LOR:
		left, leftTag, ok := comparisons(pass, e.X)
		if !ok {
			return nil, nil, false
		}
		right, rightTag, ok := comparisons(pass, e.Y)
		if !ok {
			return nil, nil, false
		}
		if !astexpr.SamePure(pass, leftTag, rightTag) {
			return nil, nil, false
		}
		return append(left, right...), leftTag, true
	}
	return nil, nil, false
}

// untyped reports whether e would take a different type as a switch
// expression than it took in the comparison. The spec converts an untyped
// operand to the type of the value beside it, where a switch converts it to
// the default type of its kind (https://go.dev/ref/spec#Expression_switches),
// so a case of the other type would not compile. The predeclared nil cannot be
// a switch expression at all, and the spec says so in the same place.
func untyped(pass *analysis.Pass, e ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok {
		return false
	}
	if untypedType(tv.Type) {
		return true // nil
	}
	if tv.Value == nil {
		return false
	}
	// A constant expression is untyped when nothing in it names a type.
	return !typedOperand(pass, e)
}

// typedOperand reports whether a constant expression names a typed value, as
// rate*2 does when rate is a typed constant, or len("ab") does through its
// result.
func typedOperand(pass *analysis.Pass, e ast.Expr) bool {
	var typed bool
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			typed = typed || typedCall(pass, n)
		case *ast.Ident:
			typed = typed || typedObject(pass, n)
		case *ast.SelectorExpr:
			typed = typed || typedObject(pass, n.Sel)
		}
		return !typed
	})
	return typed
}

// typedCall reports whether a call inside a constant expression gives its
// result a type. A conversion and len or cap do; real, imag, complex, min, and
// max return an untyped constant.
func typedCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if pass.TypesInfo.Types[call.Fun].IsType() {
		return true
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return true
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok {
		return true
	}
	switch builtin.Name() {
	case "real", "imag", "complex", "min", "max":
		return false
	}
	return true
}

// typedObject reports whether id names a value that has a type of its own. A
// builtin, a type, and a package name are not values the expression reads.
func typedObject(pass *analysis.Pass, id *ast.Ident) bool {
	obj := pass.TypesInfo.ObjectOf(id)
	switch obj.(type) {
	case nil, *types.Builtin, *types.TypeName, *types.PkgName:
		return false
	}
	return !untypedType(obj.Type())
}

// untypedType reports whether t is an untyped basic type.
func untypedType(t types.Type) bool {
	basic, ok := types.Unalias(t).(*types.Basic)
	return ok && basic.Info()&types.IsUntyped != 0
}

// captures reports whether a bare break in the branches of stmt would bind to
// the switch the fix writes rather than to the loop or switch that holds the
// if.
func captures(stmt *ast.IfStmt) bool {
	if breaksIn(stmt.Body) {
		return true
	}
	block, ok := stmt.Else.(*ast.BlockStmt)
	return ok && breaksIn(block)
}

// breaksIn reports whether node holds a bare break that a switch around it
// would take. A break inside a loop, switch, or select in node binds there
// already, and a labeled break names its own statement.
func breaksIn(node ast.Node) bool {
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil || found {
			return false
		}
		switch n := n.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.SelectStmt:
			return false
		case *ast.BranchStmt:
			found = n.Tok == token.BREAK && n.Label == nil
		}
		return !found
	})
	return found
}

// ordered reports whether a run of integer-literal clauses is written in an
// order caseorder accepts. A contiguous run in neither direction would be
// reported, and no fix rearranges a clause. The run is read the way caseorder
// reads one: a clause of one integer literal, so a character literal and a
// named constant are left out of it here too.
func ordered(pass *analysis.Pass, branches []branch) bool {
	var values []int64
	for _, b := range branches {
		if len(b.values) != 1 {
			return true
		}
		lit, ok := ast.Unparen(b.values[0]).(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return true
		}
		tv, ok := pass.TypesInfo.Types[lit]
		if !ok || tv.Value == nil {
			return true
		}
		// A floating-point tag records the literal as a Float.
		n, ok := constant.Int64Val(constant.ToInt(tv.Value))
		if !ok {
			return true
		}
		values = append(values, n)
	}
	if len(values) < 2 {
		return true
	}
	lo, hi := values[0], values[0]
	for _, n := range values[1:] {
		lo, hi = min(lo, n), max(hi, n)
	}
	if hi-lo+1 != int64(len(values)) {
		// A run with a gap is not one caseorder reads as a run.
		return true
	}
	ascending, descending := true, true
	for i := 1; i < len(values); i++ {
		ascending = ascending && values[i-1] < values[i]
		descending = descending && values[i-1] > values[i]
	}
	return ascending || descending
}

// rewrite returns the edits that write the chain as a switch, and reports
// whether it can build them.
func rewrite(pass *analysis.Pass, file *ast.File, branches []branch, tag ast.Expr) ([]analysis.TextEdit, bool) {
	tokFile := pass.Fset.File(branches[0].stmt.Pos())
	if tokFile == nil {
		return nil, false
	}
	src, err := pass.ReadFile(tokFile.Name())
	// The offsets index the bytes the driver parsed, so a file that cannot be
	// read, or that changed size, is not used.
	if err != nil || tokFile.Size() != len(src) {
		return nil, false
	}
	indent, ok := indentation(src, tokFile, branches[0].stmt)
	if !ok || !ordered(pass, branches) {
		return nil, false
	}
	clauses := make([]string, len(branches))
	for i, b := range branches {
		// The clause takes the brace's place, so the body has to start a
		// line of its own.
		if !startsBelow(tokFile, b.stmt.Body) {
			return nil, false
		}
		clauses[i], ok = clause(indent, b.values)
		if !ok {
			return nil, false
		}
	}
	first := branches[0]
	header := "switch " + tagText(tag) + " {\n" + indent + clauses[0]
	edits := []analysis.TextEdit{{
		Pos:     first.stmt.If,
		End:     first.stmt.Body.Lbrace + 1,
		NewText: []byte(header),
	}}
	for i := 1; i < len(branches); i++ {
		prev, next := branches[i-1].stmt, branches[i].stmt
		if !beginsLine(src, tokFile, prev.Body.Rbrace) {
			return nil, false
		}
		edits = append(edits,
			analysis.TextEdit{Pos: prev.Body.Rbrace, End: next.If},
			analysis.TextEdit{
				Pos:     next.If,
				End:     next.Body.Lbrace + 1,
				NewText: []byte(clauses[i]),
			},
		)
	}
	last := branches[len(branches)-1].stmt
	if block, ok := last.Else.(*ast.BlockStmt); ok {
		if !beginsLine(src, tokFile, last.Body.Rbrace) {
			return nil, false
		}
		edits = append(edits,
			analysis.TextEdit{Pos: last.Body.Rbrace, End: block.Lbrace},
			analysis.TextEdit{
				Pos:     block.Lbrace,
				End:     block.Lbrace + 1,
				NewText: []byte("default:"),
			},
		)
	}
	for _, edit := range edits {
		if astexpr.Commented(file, edit.Pos, edit.End) {
			return nil, false
		}
	}
	return edits, true
}

// cases returns a case clause's values as gofmt writes them.
func cases(values []ast.Expr) string {
	text := make([]string, len(values))
	for i, value := range values {
		text[i] = astexpr.Text(ast.Unparen(value))
	}
	return strings.Join(text, ", ")
}

// clause returns a case clause written after indent, wrapped one value per
// line when it does not fit on one, and reports false when a line is still
// over linelen's limit.
func clause(indent string, values []ast.Expr) (string, bool) {
	one := "case " + cases(values) + ":"
	if linelen.CheckLine([]byte(indent + one)) {
		return one, true
	}
	lines := make([]string, len(values))
	for i, value := range values {
		end := ","
		if i == len(values)-1 {
			end = ":"
		}
		lines[i] = astexpr.Text(ast.Unparen(value)) + end
	}
	wrapped := "case " + strings.Join(lines, "\n"+indent+"\t")
	first := true
	for line := range strings.SplitSeq(wrapped, "\n") {
		prefix := indent
		if !first {
			prefix += "\t"
		}
		first = false
		if !linelen.CheckLine([]byte(prefix + line)) {
			return "", false
		}
	}
	return wrapped, true
}

// tagText returns the expression the switch reads. A composite literal takes
// parentheses in a switch header, which cannot begin with one.
func tagText(tag ast.Expr) string {
	tag = ast.Unparen(tag)
	if astexpr.Composite(tag) {
		return "(" + astexpr.Text(tag) + ")"
	}
	return astexpr.Text(tag)
}

// indentation returns the whitespace before stmt on its line, which the case
// clauses follow, and reports false when something else is there.
func indentation(src []byte, tokFile *token.File, stmt *ast.IfStmt) (string, bool) {
	if !beginsLine(src, tokFile, stmt.If) {
		return "", false
	}
	off := tokFile.Offset(stmt.If)
	return string(src[source.Start(src, off):off]), true
}

// beginsLine reports whether nothing but whitespace precedes pos on its line.
func beginsLine(src []byte, tokFile *token.File, pos token.Pos) bool {
	off := tokFile.Offset(pos)
	return source.Blank(src[source.Start(src, off):off])
}

// startsBelow reports whether the first statement of body begins a line of its
// own, which leaves the clause's line with nothing after its colon.
func startsBelow(tokFile *token.File, body *ast.BlockStmt) bool {
	if len(body.List) == 0 {
		return true
	}
	return tokFile.Line(body.List[0].Pos()) != tokFile.Line(body.Lbrace)
}
