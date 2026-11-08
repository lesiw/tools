// Package timeafter provides an analysis.Analyzer that reports
// timer ceremony a channel receive can replace.
//
// One check: a time.NewTimer whose only uses are at most a deferred
// Stop and a single <-timer.C receive in a select sharing the
// declaration's statement list can be replaced by time.After in the receive.
// Since Go 1.23 the collector reclaims unreferenced timers before
// they fire, so the historical leak that justified the ceremony is
// gone. Statements may run between the declaration and the select;
// moving the deadline into the receive starts it later, which is
// the author's call. A timer that is Stopped conditionally, Reset, or received
// elsewhere is a real timer and is left alone, as is a select
// nested deeper than the declaration — inside a loop, most
// commonly — where time.After would restart the deadline every
// iteration. Only the timer := time.NewTimer(d) form is examined.
package timeafter

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "timeafter",
	Doc:  "reports timer ceremony that can be replaced by time.After",
	Run:  run,
}

// timer is a NewTimer variable and the shape of its uses.
type timer struct {
	recv   *ast.UnaryExpr // <-timer.C
	parent ast.Node       // the node holding the declaration
	other  bool           // any use outside the shape
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		checkFile(pass, f)
	}
	return nil, nil
}

func checkFile(pass *analysis.Pass, f *ast.File) {
	var (
		stack []ast.Node
		found []*timer

		info  = pass.TypesInfo
		byObj = make(map[types.Object]*timer)
	)
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		if assign, ok := n.(*ast.AssignStmt); ok {
			if t, obj := collectDecl(pass, assign, stack); t != nil {
				byObj[obj] = t
				found = append(found, t)
			}
			return true
		}
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		t := byObj[info.Uses[id]]
		if t == nil {
			return true
		}
		classify(t, stack)
		return true
	})
	for _, t := range found {
		if t.other || t.recv == nil {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     t.recv.Pos(),
			End:     t.recv.End(),
			Message: "timer ceremony can be replaced by time.After",
		})
	}
}

// collectDecl returns the timer of a timer := time.NewTimer(d)
// declaration, or nil.
func collectDecl(pass *analysis.Pass, assign *ast.AssignStmt, stack []ast.Node) (*timer, types.Object) {
	if assign.Tok != token.DEFINE {
		return nil, nil
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, nil
	}
	id, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, nil
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return nil, nil
	}
	if calleePath(pass, call) != "time.NewTimer" {
		return nil, nil
	}
	obj := pass.TypesInfo.Defs[id]
	if obj == nil || len(stack) < 2 {
		return nil, nil
	}
	return &timer{parent: stack[len(stack)-2]}, obj
}

// classify buckets one use of the timer by shape: a deferred Stop in
// the same list, or a <-timer.C comm clause of a select whose
// statement is a sibling of the declaration.
func classify(t *timer, stack []ast.Node) {
	if len(stack) < 2 {
		t.other = true
		return
	}
	sel, ok := stack[len(stack)-2].(*ast.SelectorExpr)
	if !ok {
		t.other = true
		return
	}
	switch sel.Sel.Name {
	case "Stop":
		t.classifyStop(stack)
	case "C":
		t.classifyRecv(stack)
	default:
		t.other = true
	}
}

func (t *timer) classifyStop(stack []ast.Node) {
	if len(stack) < 5 {
		t.other = true
		return
	}
	d, ok := stack[len(stack)-4].(*ast.DeferStmt)
	if !ok || d.Call.Fun != stack[len(stack)-2] {
		t.other = true // the timer flows into the deferred call
		return
	}
	if stack[len(stack)-5] != t.parent {
		t.other = true
	}
}

func (t *timer) classifyRecv(stack []ast.Node) {
	if len(stack) < 8 || t.recv != nil {
		t.other = true
		return
	}
	recv, ok := stack[len(stack)-3].(*ast.UnaryExpr)
	if !ok || recv.Op != token.ARROW {
		t.other = true
		return
	}
	stmt, ok := stack[len(stack)-4].(ast.Stmt)
	if !ok {
		t.other = true
		return
	}
	clause, ok := stack[len(stack)-5].(*ast.CommClause)
	if !ok || clause.Comm != stmt {
		t.other = true
		return
	}
	if stack[len(stack)-8] != t.parent {
		t.other = true
		return
	}
	t.recv = recv
}

// calleePath returns the package-qualified name of a called package
// function.
func calleePath(pass *analysis.Pass, call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	pkg, ok := pass.TypesInfo.Uses[id].(*types.PkgName)
	if !ok {
		return ""
	}
	return pkg.Imported().Path() + "." + sel.Sel.Name
}
