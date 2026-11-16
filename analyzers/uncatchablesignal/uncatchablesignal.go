// Package uncatchablesignal provides an analysis.Analyzer that reports a
// signal.Notify or signal.NotifyContext call that names SIGKILL or SIGSTOP.
//
// The os/signal package documentation says the two signals "may not be caught
// by a program, and therefore cannot be affected by this package"
// (https://pkg.go.dev/os/signal). Neither signal can be caught, so
// registering a handler for one changes nothing.
//
// The rule covers syscall.SIGKILL, syscall.SIGSTOP, and os.Kill. It follows a
// signal through parentheses and a conversion to os.Signal or syscall.Signal;
// a conversion to any other type is left alone. A signal is recognized by the
// object it names, so one copied into a local variable, as in sig := os.Kill,
// returned by a function call, or held in a slice, is left alone. The rule
// covers signal.Notify and signal.NotifyContext, the calls that relay a
// signal to a channel; signal.Ignore and signal.Reset are left alone.
//
// The fix removes the uncatchable signals and the comma that joins them to a
// neighbor. A call that names several uncatchable signals is reported once. A
// call whose signal arguments are all uncatchable is reported without a fix,
// because removing every signal would register for every signal. Deleting the
// call is not offered because the channel or context expression may have
// effects and signal.NotifyContext returns values the caller uses.
//
// Fixable when a catchable signal remains and no comment sits in or next to
// the removed text; report only otherwise, because the edit would delete the
// comment or leave it describing another signal.
//
// On Plan 9, os.Kill and syscall.SIGKILL are the "kill" note, which the
// runtime handles without notifying os/signal, so the rule applies there too.
//
// The rule is adapted from staticcheck's SA1016, which also reports
// signal.Ignore and signal.Reset.
package uncatchablesignal

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "uncatchablesignal",
	Doc:  "reports signal.Notify or NotifyContext on SIGKILL or SIGSTOP",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports one signal registration that names SIGKILL or SIGSTOP, and
// attaches a fix that drops the uncatchable signals when a safe edit exists.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "os/signal" {
		return
	}
	if fn.Name() != "Notify" && fn.Name() != "NotifyContext" {
		return
	}
	if len(call.Args) == 0 {
		return
	}
	signals := call.Args[1:]
	var bad []int
	var names []string
	for i, signal := range signals {
		name, ok := uncatchable(pass, signal)
		if !ok {
			continue
		}
		bad = append(bad, i)
		names = append(names, name)
	}
	if len(bad) == 0 {
		return
	}
	names = unique(names)
	d := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: join(names) + " cannot be caught",
	}
	edits, ok := removals(signals, bad)
	if ok && !commented(file, call, bad) {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Remove " + join(names),
			TextEdits: edits,
		}}
	}
	pass.Report(d)
}

// uncatchable returns the source text of a signal argument that names SIGKILL
// or SIGSTOP, looking through parentheses and a conversion to os.Signal or
// syscall.Signal.
func uncatchable(pass *analysis.Pass, expr ast.Expr) (string, bool) {
	expr = ast.Unparen(expr)
	if call, ok := expr.(*ast.CallExpr); ok && len(call.Args) == 1 {
		tv := pass.TypesInfo.Types[call.Fun]
		if tv.IsType() && signalType(tv.Type) {
			return uncatchable(pass, call.Args[0])
		}
	}
	var obj types.Object
	switch expr := expr.(type) {
	case *ast.Ident:
		obj = pass.TypesInfo.ObjectOf(expr)
	case *ast.SelectorExpr:
		obj = pass.TypesInfo.ObjectOf(expr.Sel)
	}
	if obj == nil || obj.Pkg() == nil {
		return "", false
	}
	// On Plan 9, os.Kill and syscall.SIGKILL are the "kill" note. The
	// runtime table marks it _SigKill without _SigNotify, so it cannot be
	// caught.
	switch obj.Pkg().Path() + "." + obj.Name() {
	case "os.Kill", "syscall.SIGKILL", "syscall.SIGSTOP":
		return astexpr.Text(expr), true
	}
	return "", false
}

func signalType(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	switch named.Obj().Pkg().Path() + "." + named.Obj().Name() {
	case "os.Signal", "syscall.Signal":
		return true
	}
	return false
}

type badRun struct{ first, last int }

// runs groups adjacent indexes, so one edit can take a run with its joining
// comma.
func runs(bad []int) (out []badRun) {
	for start := 0; start < len(bad); {
		last := start
		for last+1 < len(bad) && bad[last+1] == bad[last]+1 {
			last++
		}
		out = append(out, badRun{first: bad[start], last: bad[last]})
		start = last + 1
	}
	return out
}

// removals returns the edits that delete the signals at the given indexes,
// each with the comma that joins it to a neighbor. It reports false when every
// signal is uncatchable, since removing them all would leave an empty list,
// which registers for every signal.
func removals(signals []ast.Expr, bad []int) ([]analysis.TextEdit, bool) {
	if len(bad) == len(signals) {
		return nil, false
	}
	var edits []analysis.TextEdit
	for _, run := range runs(bad) {
		if run.last+1 < len(signals) {
			edits = append(edits, analysis.TextEdit{
				Pos: signals[run.first].Pos(),
				End: signals[run.last+1].Pos(),
			})
		} else {
			edits = append(edits, analysis.TextEdit{
				Pos: signals[run.first-1].End(),
				End: signals[run.last].End(),
			})
		}
	}
	return edits, true
}

// commented reports whether a comment sits beside a removed signal. The
// window reaches one argument on each side because the edit takes the joining
// comma with it.
func commented(file *ast.File, call *ast.CallExpr, bad []int) bool {
	signals := call.Args[1:]
	for _, run := range runs(bad) {
		lo := call.Args[0].End()
		if run.first > 0 {
			lo = signals[run.first-1].End()
		}
		hi := call.Rparen
		if run.last+1 < len(signals) {
			hi = signals[run.last+1].Pos()
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if comment.Pos() < hi && comment.End() > lo {
					return true
				}
			}
		}
	}
	return false
}

// unique keeps a signal named twice from appearing twice in the message.
func unique(names []string) []string {
	var (
		seen = make(map[string]struct{}, len(names))
		out  = names[:0]
	)
	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func join(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	}
	return strings.Join(names[:len(names)-1], ", ") + ", and " +
		names[len(names)-1]
}
