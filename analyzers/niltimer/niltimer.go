// Package niltimer provides an analysis.Analyzer that reports a method call
// on a nil *time.Timer or *time.Ticker.
//
// The Timer documentation says a Timer must be created with NewTimer or
// AfterFunc. Each Timer and Ticker method reads a field of the receiver:
// "if !t.initTimer" in Timer.Stop and Timer.Reset, and "if !t.initTicker" in
// Ticker.Stop and Ticker.Reset (https://go.dev/src/time/sleep.go,
// https://go.dev/src/time/tick.go). A nil receiver panics during the call.
// TestIssue5745 in the time package calls Stop on a nil Timer to exercise the
// panic (https://go.dev/src/time/sleep_test.go).
//
// The rule reports a call when the receiver's value is nil. A variable copied
// from another is still nil, and so is a merge of nil values. A variable
// assigned a timer on one branch before the call is not reported. A named
// result starts nil. Without a defer in the function, a call on one is
// reported when no assignment reaches it; a defer moves the result to memory,
// which the rule does not follow. A nil constant, a conversion of nil, and a
// merge of nil values are recognized. A load from memory is not: a field, a
// package variable, a collection element, and a channel receive are left
// alone. An incoming parameter, a call result, and a type assertion are not
// resolved by the rule. A receiver whose address is taken, or one that a
// function literal captures, is left alone too, since code the rule does not
// follow may set it first. A call through an interface value is left alone,
// since the implementation that runs is not known until run time. A stored
// method value is left alone, since it goes through a closure that captures
// the receiver; an immediate method value is reported, since it lowers to a
// direct call. A method expression is reported when called directly or through
// a local value; one loaded from a package variable or field is left alone.
//
// A call in a package-level variable initializer, including a function literal
// there, is left alone, since the package initialization function is not
// analyzed.
//
// A call inside a nil test is still reported, even when the condition is
// always false.
//
// Report only: two repairs are plausible, deleting the call or guarding it,
// and the choice belongs to the author.
//
// The rule is adapted from staticcheck's SA1019 family.
package niltimer

import (
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:     "niltimer",
	Doc:      "reports a Timer or Ticker method call on a nil pointer",
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	info := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	for _, fn := range info.SrcFuncs {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				var call *ssa.CallCommon
				switch instr := instr.(type) {
				case *ssa.Call:
					call = &instr.Call
				case *ssa.Defer:
					call = &instr.Call
				case *ssa.Go:
					call = &instr.Call
				default:
					continue
				}
				check(pass, call)
			}
		}
	}
	return nil, nil
}

// check reports call when it is a Timer or Ticker method call whose receiver
// is nil.
func check(pass *analysis.Pass, call *ssa.CallCommon) {
	if call.IsInvoke() {
		return
	}
	callee := call.StaticCallee()
	if callee == nil || len(call.Args) == 0 {
		return
	}
	fn, ok := callee.Object().(*types.Func)
	if !ok {
		return
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return
	}
	name, ok := timerType(sig.Recv().Type())
	if !ok || !nilValue(call.Args[0]) {
		return
	}
	pass.Reportf(call.Pos(), "call to %s on a nil %s", fn.Name(), name)
}

// timerType returns Timer or Ticker when t is the pointer type *time.Timer or
// *time.Ticker.
func timerType(t types.Type) (string, bool) {
	ptr, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return "", false
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
	if !ok {
		return "", false
	}
	obj := named.Obj()
	if obj.Pkg() == nil || obj.Pkg().Path() != "time" {
		return "", false
	}
	switch obj.Name() {
	case "Timer", "Ticker":
		return obj.Name(), true
	}
	return "", false
}

// nilValue reports whether v holds nil on every path that reaches it. A value
// already seen is treated as nil, so a merge that leads back to itself needs
// no separate guard. Only a merge can close a cycle. The walk also appears in
// nilmapassign, with fewer conversions; the two must agree.
func nilValue(v ssa.Value) bool {
	var (
		seen = make(map[ssa.Value]struct{})
		walk func(ssa.Value) bool
	)
	walk = func(v ssa.Value) bool {
		if _, ok := seen[v]; ok {
			return true
		}
		seen[v] = struct{}{}
		switch v := v.(type) {
		case *ssa.Phi:
			for _, edge := range v.Edges {
				if !walk(edge) {
					return false
				}
			}
			return true
		case *ssa.ChangeType:
			return walk(v.X)
		case *ssa.Convert:
			return walk(v.X)
		case *ssa.Const:
			return v.Value == nil
		}
		return false
	}
	return walk(v)
}
