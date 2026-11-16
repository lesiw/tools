// Package badlock provides an analysis.Analyzer that reports a deferred call
// that does not release the lock taken in the statement before it.
//
// Unlock's documentation says "It is a run-time error if m is not locked on
// entry to Unlock", and RWMutex's says "A RWMutex.RLock cannot be upgraded
// into a RWMutex.Lock, nor can a RWMutex.Lock be downgraded into a
// RWMutex.RLock". A deferred release that names the other method is a run-time
// error. A deferred call that takes a lock waits for the lock the function
// already holds, and a deferred RLock after an RLock waits only when a writer
// is blocked, and otherwise leaves a read lock that nothing releases.
//
// The fix replaces the deferred method with the release of the lock that was
// taken, and it is withheld where the receiver's type has no method with the
// release's name that takes no arguments, or where the rewrite would take the
// line past the width linelen measures.
//
// The README catalogs every check, exemption, and fix.
//
// The rule is adapted from go-critic's badLock.
package badlock

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "badlock",
	Doc:  "reports a deferred call that does not release the lock taken in the statement before it",
	Run:  run,
}

// releases maps the method that takes a lock to the method that releases it.
// TryLock and TryRLock are left out because each may return without the lock.
var releases = map[string]string{
	"Lock":  "Unlock",
	"RLock": "RUnlock",
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BlockStmt:
				report(pass, n.List)
			case *ast.CaseClause:
				report(pass, n.Body)
			case *ast.CommClause:
				report(pass, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// report reads one statement list, two statements at a time.
func report(pass *analysis.Pass, list []ast.Stmt) {
	for i := 0; i+1 < len(list); i++ {
		stmt, ok := list[i].(*ast.ExprStmt)
		if !ok {
			continue
		}
		took, ok := methodCall(pass, stmt.X)
		if !ok {
			continue
		}
		release, ok := releases[took.Sel.Name]
		if !ok {
			continue
		}
		deferStmt, ok := list[i+1].(*ast.DeferStmt)
		if !ok {
			continue
		}
		deferred, ok := methodCall(pass, deferStmt.Call)
		if !ok {
			continue
		}
		if !astexpr.Same(pass, took.X, deferred.X) {
			continue
		}
		switch deferred.Sel.Name {
		case release:
			continue
		case "Lock", "RLock", "Unlock", "RUnlock":
			pass.Report(diagnostic(pass, deferred, took, release))
		}
	}
}

// diagnostic builds the report for a deferred call that does not release the
// lock taken, with a fix where the release can be called on the receiver and
// the rewrite keeps its line within the width.
func diagnostic(pass *analysis.Pass, deferred, took *ast.SelectorExpr, release string) analysis.Diagnostic {
	d := analysis.Diagnostic{
		Pos: deferred.Pos(),
		End: deferred.End(),
		Message: "the deferred " + deferred.Sel.Name +
			" does not release the lock taken by " + took.Sel.Name,
	}
	if !hasRelease(pass, deferred.X, release) {
		return d
	}
	if !fits(pass, deferred.Sel.Pos(), deferred.Sel.End(), release) {
		return d
	}
	d.SuggestedFixes = []analysis.SuggestedFix{{
		Message: "Replace with " + release,
		TextEdits: []analysis.TextEdit{{
			Pos:     deferred.Sel.Pos(),
			End:     deferred.Sel.End(),
			NewText: []byte(release),
		}},
	}}
	return d
}

// hasRelease reports whether the type of recv has a method named name that
// takes no arguments, so that the replacement call compiles.
func hasRelease(pass *analysis.Pass, recv ast.Expr, name string) bool {
	tv, ok := pass.TypesInfo.Types[recv]
	if !ok || tv.Type == nil {
		return false
	}
	obj, _, _ := types.LookupFieldOrMethod(
		tv.Type, tv.Addressable(),
		pass.Pkg, name,
	)
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	return fn.Signature().Params().Len() == 0
}

// fits reports whether writing release over the range keeps the line within
// the width. A line already past the width fits: refusing the fix cannot
// shorten it, and linelen reports the line either way.
func fits(pass *analysis.Pass, from, to token.Pos, release string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	// Written positions, which a //line directive does not move.
	var (
		start = pass.Fset.PositionFor(from, false)
		end   = pass.Fset.PositionFor(to, false)
	)
	src, err := pass.ReadFile(start.Filename)
	if err != nil {
		return false
	}
	lines := bytes.Split(src, []byte("\n"))
	if start.Line > len(lines) {
		return false
	}
	line := lines[start.Line-1]
	// A driver that parses other bytes than ReadFile returns puts the
	// position outside the line, so the bounds are checked rather than
	// trusted.
	if start.Column > len(line)+1 || end.Column > len(line)+1 {
		return false
	}
	if !linelen.CheckLine(line) {
		return true
	}
	var rewritten []byte
	rewritten = append(rewritten, line[:start.Column-1]...)
	rewritten = append(rewritten, release...)
	rewritten = append(rewritten, line[end.Column-1:]...)
	return linelen.CheckLine(rewritten)
}

// methodCall returns the selector e calls, if e is a call of a method that
// takes no arguments. A call with an argument is left out: a lock's own
// methods take none, and the fix rewrites only the method name, so an argument
// would be left on a call that does not take it. A field of function type is
// not a method, and its name says nothing about a lock.
func methodCall(pass *analysis.Pass, e ast.Expr) (*ast.SelectorExpr, bool) {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil, false
	}
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	selection, ok := pass.TypesInfo.Selections[sel]
	if !ok || selection.Kind() != types.MethodVal {
		return nil, false
	}
	return sel, true
}
