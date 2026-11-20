// Package regexpmatchloop provides an analysis.Analyzer that reports a call
// to regexp.Match, regexp.MatchString or regexp.MatchReader in a loop body.
//
// Each of those functions compiles its pattern before matching. The regexp
// documentation for them says that a more complicated query needs Compile and
// the full Regexp interface. A constant pattern compiles the same way on every
// call, so one compile before the loop suffices.
//
// The fix hoists a regexp.MustCompile of the pattern above the loop and calls
// the compiled value. It is withheld where the call is not the whole
// statement, where the pattern does not compile, and where a comment is in the
// call's text or immediately before the loop; the README lists the other
// cases.
//
// The rule is adapted from staticcheck's SA6000.
package regexpmatchloop

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "regexpmatchloop",
	Doc:  "reports a regexp match function called in a loop with a constant pattern",
	Run:  run,
}

// matchFunctions are the functions that compile their pattern on each call.
var matchFunctions = map[string]struct{}{
	"Match":       {},
	"MatchReader": {},
	"MatchString": {},
}

// match is one call this rule reports.
type match struct {
	call      *ast.CallExpr
	sel       *ast.SelectorExpr
	loop      ast.Node
	name      string // the variable the fix declares
	anchor    ast.Node
	discarded bool // the call's results are discarded
	fix       bool // the call is in a fixable group
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var (
			parent = astexpr.Parents(file)
			used   = names(pass, file)
			found  []match
		)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			m, ok := matchOf(pass, parent, call)
			if !ok {
				return true
			}
			m.anchor = anchorOf(parent, m.loop)
			found = append(found, m)
			return true
		})
		groups := make(map[ast.Node][]match)
		for i := range found {
			if !fixable(pass, file, parent, found[i]) {
				continue
			}
			found[i].name = pickName(used)
			found[i].fix = true
			m := found[i]
			groups[m.loop] = append(groups[m.loop], m)
		}
		fixes := make(map[ast.Node]analysis.SuggestedFix, len(groups))
		for loop, ms := range groups {
			fixes[loop] = hoist(ms)
		}
		for _, m := range found {
			d := diagnose(m)
			if m.fix {
				d.SuggestedFixes = []analysis.SuggestedFix{fixes[m.loop]}
			}
			pass.Report(d)
		}
	}
	return nil, nil
}

// names returns the names that a declaration in file would shadow: every
// identifier written in the file and every name declared in the package.
func names(pass *analysis.Pass, file *ast.File) map[string]bool {
	used := make(map[string]bool)
	for _, name := range pass.Pkg.Scope().Names() {
		used[name] = true
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			used[ident.Name] = true
		}
		return true
	})
	return used
}

// pickName returns a name for a compiled pattern that no existing identifier
// holds, and reserves it for the next call.
func pickName(used map[string]bool) string {
	for i := 1; ; i++ {
		name := "re"
		if i > 1 {
			name += strconv.Itoa(i)
		}
		if !used[name] {
			used[name] = true
			return name
		}
	}
}

// matchOf returns call as a reported match, and reports false for any call
// this rule leaves alone.
func matchOf(pass *analysis.Pass, parent map[ast.Node]ast.Node, call *ast.CallExpr) (match, bool) {
	if len(call.Args) != 2 {
		return match{}, false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return match{}, false
	}
	if _, ok := matchFunctions[sel.Sel.Name]; !ok {
		return match{}, false
	}
	ident, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return match{}, false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok || pkg.Imported().Path() != "regexp" {
		return match{}, false
	}
	tv, ok := pass.TypesInfo.Types[call.Args[0]]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return match{}, false
	}
	loop, ok := enclosingLoop(parent, call)
	if !ok {
		return match{}, false
	}
	return match{
		call:      call,
		sel:       sel,
		loop:      loop,
		discarded: discarded(parent, call),
	}, true
}

// enclosingLoop returns the loop whose body holds call. It reports false for
// a call outside a loop body, and for a call in a function literal that holds
// no loop of its own.
func enclosingLoop(parent map[ast.Node]ast.Node, call ast.Node) (ast.Node, bool) {
	for n := parent[call]; n != nil; n = parent[n] {
		switch n := n.(type) {
		case *ast.FuncLit:
			return nil, false
		case *ast.ForStmt:
			if within(n.Body, call) {
				return n, true
			}
		case *ast.RangeStmt:
			if within(n.Body, call) {
				return n, true
			}
		}
	}
	return nil, false
}

// discarded reports whether call's results are thrown away, as in an
// expression statement, a go statement, or a defer statement.
func discarded(parent map[ast.Node]ast.Node, call ast.Node) bool {
	for {
		next := parent[call]
		if paren, ok := next.(*ast.ParenExpr); ok {
			call = paren
			continue
		}
		switch next.(type) {
		case *ast.ExprStmt, *ast.GoStmt, *ast.DeferStmt:
			return true
		}
		return false
	}
}

// within reports whether node lies inside body.
func within(body *ast.BlockStmt, node ast.Node) bool {
	return body.Pos() <= node.Pos() && node.End() <= body.End()
}

// anchorOf returns the node the fix writes before: the loop, or a label on it.
func anchorOf(parent map[ast.Node]ast.Node, loop ast.Node) ast.Node {
	for {
		label, ok := parent[loop].(*ast.LabeledStmt)
		if !ok || label.Stmt != loop {
			return loop
		}
		loop = label
	}
}

// diagnose returns the diagnostic for one match.
func diagnose(m match) analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos: m.call.Pos(),
		End: m.call.End(),
		Message: "regexp." + m.sel.Sel.Name +
			" compiles the pattern on each call; compile it once before the loop",
	}
}

// hoist returns the fix that declares every pattern of one loop above it and
// calls the compiled value. Every match in the loop carries the same fix, so
// a driver applies it once.
func hoist(ms []match) analysis.SuggestedFix {
	var (
		decls strings.Builder
		edits = []analysis.TextEdit{{
			Pos: ms[0].anchor.Pos(),
			End: ms[0].anchor.Pos(),
		}}
	)
	for _, m := range ms {
		var (
			pattern = astexpr.Text(m.call.Args[0])
			qual    = ast.Unparen(m.sel.X).(*ast.Ident).Name
		)
		decls.WriteString(m.name + " := " + qual + ".MustCompile(")
		decls.WriteString(pattern + ")\n")
		edits = append(edits, analysis.TextEdit{
			Pos:     m.sel.Pos(),
			End:     m.sel.End(),
			NewText: []byte(m.name + "." + m.sel.Sel.Name),
		}, analysis.TextEdit{
			Pos: m.call.Args[0].Pos(),
			End: m.call.Args[1].Pos(),
		})
	}
	edits[0].NewText = []byte(decls.String())
	message := "Compile the pattern once before the loop"
	if len(ms) > 1 {
		message = "Compile the patterns once before the loop"
	}
	return analysis.SuggestedFix{
		Message:   message,
		TextEdits: edits,
	}
}

// fixable reports whether the hoist can be written without losing a comment
// or changing where the pattern comes from.
func fixable(pass *analysis.Pass, file *ast.File, parent map[ast.Node]ast.Node, m match) bool {
	var (
		call = commented(file, m.call.Pos(), m.call.End())
		near = attached(pass, file, m.anchor.Pos())
	)
	if call || near || !m.discarded {
		return false
	}
	if jumpedOver(parent, m.anchor) {
		return false
	}
	tv := pass.TypesInfo.Types[m.call.Args[0]]
	if _, err := regexp.Compile(constant.StringVal(tv.Value)); err != nil {
		return false
	}
	return inScope(pass, m.call.Args[0], m.anchor.Pos())
}

// inScope reports whether every name the pattern reads has the same object at
// pos, where the fix writes the declaration.
func inScope(pass *analysis.Pass, pattern ast.Expr, pos token.Pos) bool {
	skip := make(map[*ast.Ident]struct{})
	ast.Inspect(pattern, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			skip[sel.Sel] = struct{}{}
		}
		return true
	})
	ok := true
	ast.Inspect(pattern, func(n ast.Node) bool {
		ident, isIdent := n.(*ast.Ident)
		if !isIdent {
			return true
		}
		if _, ok := skip[ident]; ok {
			return true
		}
		obj := pass.TypesInfo.Uses[ident]
		if obj == nil || obj.Parent() == types.Universe {
			return true
		}
		if imports.Lookup(pass, ident.Name, pos) != obj {
			ok = false
		}
		return ok
	})
	return ok
}

// functionBody returns the body of the function that holds node.
func functionBody(parent map[ast.Node]ast.Node, node ast.Node) *ast.BlockStmt {
	for n := node; n != nil; n = parent[n] {
		switch n := n.(type) {
		case *ast.FuncDecl:
			return n.Body
		case *ast.FuncLit:
			return n.Body
		}
	}
	return nil
}

// jumpedOver reports whether a goto in the function jumps from before anchor
// to a label at or after it, which Go rejects when anchor is a declaration.
func jumpedOver(parent map[ast.Node]ast.Node, anchor ast.Node) bool {
	body := functionBody(parent, anchor)
	if body == nil {
		return false
	}
	labels := make(map[string]token.Pos)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.LabeledStmt:
			labels[n.Label.Name] = n.Label.Pos()
		}
		return true
	})
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if n.Tok != token.GOTO {
				return true
			}
			target, ok := labels[n.Label.Name]
			if ok && n.Pos() < anchor.Pos() && target >= anchor.Pos() {
				found = true
			}
		}
		return !found
	})
	return found
}

// commented reports whether a comment overlaps the text between lo and hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}

// attached reports whether a comment that starts before pos ends on pos's
// line or the line above it, where the declaration would separate it from the
// loop.
func attached(pass *analysis.Pass, file *ast.File, pos token.Pos) bool {
	line := pass.Fset.Position(pos).Line
	for _, group := range file.Comments {
		if group.Pos() >= pos {
			continue
		}
		end := pass.Fset.Position(group.End()).Line
		if end == line || end == line-1 {
			return true
		}
	}
	return false
}
