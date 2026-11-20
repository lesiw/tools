// Package builtinbound provides an analysis.Analyzer that reports a
// comparison of len or cap with zero that is always true or always false.
//
// The Length and capacity section of the spec gives 0 <= len(s) <= cap(s) for
// a slice, and gives the length of a string, a map and a channel as a count,
// so neither builtin returns a negative number
// (https://go.dev/ref/spec#Length_and_capacity). len(x) < 0 and 0 > len(x) are
// always false, len(x) >= 0 and 0 <= len(x) are always true, and cap is
// reported the same way.
//
// The rule is adapted from staticcheck's SA4024, which reports the comparisons
// that are always false for len and cap, and from go-critic's sloppyLen, which
// reports the ones that are always true for len.
package builtinbound

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "builtinbound",
	Doc:  "reports a comparison of len or cap with zero that is always true or always false",
	Run:  run,
}

// checker holds the reads of every object in the pass, the signature ranges
// that may go unread, and the reads the fixes kept so far delete.
type checker struct {
	pass     *analysis.Pass
	reads    map[types.Object]int
	sigs     []ast.Node
	findings []finding
	dropped  map[types.Object]int
}

// finding is a comparison whose answer cannot vary.
type finding struct {
	file    *ast.File
	bin     *ast.BinaryExpr
	operand ast.Expr
	name    string
	result  bool
}

func run(pass *analysis.Pass) (any, error) {
	c := &checker{pass: pass, dropped: make(map[types.Object]int)}
	for _, file := range pass.Files {
		c.collect(file)
	}
	// Only a fix reads these.
	if len(c.findings) == 0 {
		return nil, nil
	}
	c.reads, c.sigs = reads(pass), signatures(pass)
	for i := range c.findings {
		c.report(&c.findings[i])
	}
	return nil, nil
}

// collect appends the comparisons in file that the rule reports.
func (c *checker) collect(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		bin, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		if f, ok := findingFor(c.pass, file, bin); ok {
			c.findings = append(c.findings, f)
		}
		return true
	})
}

// findingFor returns the finding for a comparison of a builtin's result with
// zero, when the answer cannot vary.
func findingFor(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr) (finding, bool) {
	var (
		name, operand, ok = builtinOperand(pass, bin.X, bin.Y)
		op                = bin.Op
	)
	if !ok {
		name, operand, ok = builtinOperand(pass, bin.Y, bin.X)
		op = astexpr.Flip(op)
	}
	if !ok {
		return finding{}, false
	}
	var result bool
	switch op {
	case token.LSS:
		result = false
	case token.GEQ:
		result = true
	default:
		return finding{}, false
	}
	return finding{
		file:    file,
		bin:     bin,
		operand: operand,
		name:    name,
		result:  result,
	}, true
}

// builtinOperand returns the name and operand of a call to the builtin len or
// cap over e, when zero is the literal 0. Parentheses are ignored on either
// side.
func builtinOperand(pass *analysis.Pass, e, zero ast.Expr) (string, ast.Expr, bool) {
	lit, ok := ast.Unparen(zero).(*ast.BasicLit)
	if !ok || lit.Kind != token.INT || lit.Value != "0" {
		return "", nil, false
	}
	expr, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(expr.Args) != 1 {
		return "", nil, false
	}
	id, ok := ast.Unparen(expr.Fun).(*ast.Ident)
	if !ok {
		return "", nil, false
	}
	obj := pass.TypesInfo.Uses[id]
	if obj == types.Universe.Lookup("len") {
		return "len", expr.Args[0], true
	}
	if obj == types.Universe.Lookup("cap") {
		return "cap", expr.Args[0], true
	}
	return "", nil, false
}

// report sends the diagnostic for f, with the constant as its fix when the
// edit is safe.
func (c *checker) report(f *finding) {
	var (
		answer = strconv.FormatBool(f.result)
		diag   = analysis.Diagnostic{
			Pos:     f.bin.Pos(),
			End:     f.bin.End(),
			Message: f.name + " is never negative; this is always " + answer,
		}
	)
	if c.fixable(f) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Write the value",
			TextEdits: []analysis.TextEdit{{
				Pos:     f.bin.Pos(),
				End:     f.bin.End(),
				NewText: []byte(answer),
			}},
		}}
	}
	c.pass.Report(diag)
}

// fixable reports whether the constant can stand in for f, and counts the
// reads that it deletes when it can.
func (c *checker) fixable(f *finding) bool {
	// Replacing the comparison deletes the operand, so the operand must not
	// run code, panic, or hold a comment.
	if !astexpr.Safe(c.pass, f.operand) {
		return false
	}
	if astexpr.Commented(f.file, f.bin.Pos(), f.bin.End()) {
		return false
	}
	// The compiler rejects a local that nothing reads, and one of these may
	// be the last read of one.
	deleted := c.deleted(f)
	for obj, n := range deleted {
		if !c.keeps(obj, n) {
			return false
		}
	}
	for obj, n := range deleted {
		c.dropped[obj] += n
	}
	return true
}

// deleted returns the reads of local variables that the constant removes.
func (c *checker) deleted(f *finding) map[types.Object]int {
	deleted := make(map[types.Object]int)
	ast.Inspect(f.operand, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if obj, ok := c.local(ident); ok {
				deleted[obj]++
			}
		}
		return true
	})
	return deleted
}

// keeps reports whether a read of obj survives the fix, along with the n reads
// that the fix removes.
func (c *checker) keeps(obj types.Object, n int) bool {
	var (
		reads = c.reads[obj]
		gone  = c.dropped[obj] + n
	)
	// A type switch declares its variable once for every clause, all at one
	// position, and a clause that reads it keeps the declaration used.
	for other, count := range c.reads {
		if other != obj && other.Pos() == obj.Pos() {
			reads += count
			gone += c.dropped[other]
		}
	}
	return reads > gone
}

// local returns the variable ident reads, when the compiler requires the
// declaration to be read. A field, a package-level variable, a parameter, a
// result and a receiver may go unread.
func (c *checker) local(ident *ast.Ident) (types.Object, bool) {
	obj, ok := c.pass.TypesInfo.Uses[ident].(*types.Var)
	if !ok || obj.IsField() {
		return nil, false
	}
	// A package-level variable may go unread, in this package or in one it
	// imports.
	if obj.Pkg() != c.pass.Pkg || obj.Parent() == c.pass.Pkg.Scope() {
		return nil, false
	}
	return obj, !c.inSignature(obj)
}

// reads returns the number of reads of every object in the package. An
// assignment's target is not a read, so x = 0 leaves x's declaration unused.
func reads(pass *analysis.Pass) map[types.Object]int {
	writes := make(map[*ast.Ident]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				if n.Tok == token.ASSIGN || n.Tok == token.DEFINE {
					targets(writes, n.Lhs)
				}
			case *ast.RangeStmt:
				targets(writes, []ast.Expr{n.Key, n.Value})
			}
			return true
		})
	}
	count := make(map[types.Object]int)
	for ident, obj := range pass.TypesInfo.Uses {
		if _, ok := writes[ident]; !ok {
			count[obj]++
		}
	}
	return count
}

// targets adds the plain identifiers among exprs to writes. Another expression
// on the left, such as a[i], is not a declaration.
func targets(writes map[*ast.Ident]struct{}, exprs []ast.Expr) {
	for _, expr := range exprs {
		if ident, ok := ast.Unparen(expr).(*ast.Ident); ok {
			writes[ident] = struct{}{}
		}
	}
}

// signatures returns the parameter, result and receiver lists of the package,
// whose declarations the compiler does not require to be read.
func signatures(pass *analysis.Pass) []ast.Node {
	var found []ast.Node
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Recv != nil {
					found = append(found, n.Recv)
				}
				found = append(found, n.Type)
			case *ast.FuncLit:
				found = append(found, n.Type)
			}
			return true
		})
	}
	return found
}

// inSignature reports whether obj is declared in one of the signatures, which
// is where a parameter, a result and a receiver are declared.
func (c *checker) inSignature(obj types.Object) bool {
	for _, sig := range c.sigs {
		if sig.Pos() <= obj.Pos() && obj.Pos() < sig.End() {
			return true
		}
	}
	return false
}
