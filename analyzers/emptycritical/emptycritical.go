// Package emptycritical provides an analysis.Analyzer that reports a lock
// taken and released with no statement between the two calls.
//
// sync.Mutex's documentation says "The zero value for a Mutex is an unlocked
// mutex", and Unlock's says "A locked Mutex is not associated with a
// particular goroutine". A section that takes the lock and gives it back
// does no work between the calls. The author often meant the Lock to cover
// the work below it, or wrote an Unlock without the defer they meant.
//
// Unlock and RUnlock also "synchronize before" the next Lock, so either pair
// can be a barrier that another goroutine waits on, and removing it would
// change the program. Report only. The rule skips a test file, because a
// benchmark takes and releases a lock to measure it and puts nothing between
// the calls. An //ignore directive suppresses the report for a deliberate
// barrier.
//
//	func f(mu *sync.Mutex) {
//		mu.Lock()
//		mu.Unlock()
//	}
//
// The rule is adapted from staticcheck's SA2001, which reports the same pair
// and offers no fix.
package emptycritical

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "emptycritical",
	Doc:  "reports a lock taken and released with no statement between",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go") {
			continue
		}
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
		took, ok := method(pass, list[i])
		if !ok {
			continue
		}
		release, ok := method(pass, list[i+1])
		if !ok || !astexpr.Same(pass, took.X, release.X) {
			continue
		}
		if !releases(took.Sel.Name, release.Sel.Name) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     release.Pos(),
			End:     release.End(),
			Message: "empty critical section",
		})
	}
}

// releases reports whether release gives back the lock that took takes.
func releases(took, release string) bool {
	switch took {
	case "Lock":
		return release == "Unlock"
	case "RLock":
		return release == "RUnlock"
	}
	return false
}

// method returns the selector stmt calls, if stmt is a call of a method that
// takes no arguments and returns no results. A field of function type is not
// a method, so the rule does not treat it as a lock.
func method(pass *analysis.Pass, stmt ast.Stmt) (*ast.SelectorExpr, bool) {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil, false
	}
	call, ok := ast.Unparen(expr.X).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	selection, ok := pass.TypesInfo.Selections[sel]
	if !ok || selection.Kind() != types.MethodVal {
		return nil, false
	}
	fn, ok := selection.Obj().(*types.Func)
	if !ok {
		return nil, false
	}
	sig := fn.Signature()
	if sig.Params().Len() != 0 || sig.Results().Len() != 0 {
		return nil, false
	}
	return sel, true
}
