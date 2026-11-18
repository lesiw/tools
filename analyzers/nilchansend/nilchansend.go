// Package nilchansend provides an analysis.Analyzer that reports a send or
// receive on a channel that is nil at that point.
//
// The specification's Channel types section says "A nil channel is never ready
// for communication" (https://go.dev/ref/spec#Channel_types). The Send
// statements section says "A send on a nil channel blocks forever"
// (https://go.dev/ref/spec#Send_statements), and the Receive operator section
// says "Receiving from a nil channel blocks forever"
// (https://go.dev/ref/spec#Receive_operator). A range over a nil channel
// blocks forever too (https://go.dev/ref/spec#For_statements).
//
// The analysis records the channel values it can prove nil at each point. A
// local variable declared without a value is nil, as is an explicit nil,
// a conversion of nil, a copy of a nil value, and a merge whose incoming
// values are all nil. A comparison with nil narrows the value to nil on the
// nil branch. A value the analysis proves non-nil is not narrowed. An
// operation is reported when the analysis proves the channel nil on every
// path to it. Each operation is reported on its own, including one after an
// operation that blocks forever; the analysis does not prove reachability. A
// loop value with an edge that is not nil is left alone, even when another
// iteration would use a nil value.
//
// These values start unknown: a parameter, a package variable, a field, an
// element, a call result, a receive, and a type assertion. A parameter and a
// local variable can be shown nil at a comparison with nil; this covers a
// call result, a receive, and a type assertion once they are assigned to a
// local variable. A field, an element, and a package
// variable are read again at each use, so a comparison does not narrow them. A
// variable whose address is taken, and one captured by a function literal, are
// left alone, because another function can write them. A named result whose
// address is taken, as by a defer, is left alone for the same reason.
//
// A select with more than one case, or a select with a default, is not
// reported at its cases. A nil channel disables that case, and a select with
// every case disabled and no default blocks in the select
// (https://go.dev/ref/spec#Select_statements). A select with a single case and
// no default becomes a plain send or receive and is reported. A send or
// receive in the body of a selected case is reported like any other.
//
// The suite's nilness pass reports some of the same operations. This rule also
// reports an operation after a branch join where every incoming value is nil.
// The cmd/vet suite runs both, so an operation that both report gets two
// diagnostics.
//
// Report only: a fix would have to choose a channel value, and removing the
// operation changes the function's synchronization.
//
// The rule is adapted from staticcheck's SA5011 family.
package nilchansend

import (
	"go/token"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

const (
	sendMessage    = "send on nil channel blocks forever"
	receiveMessage = "receive from nil channel blocks forever"
)

var Analyzer = &analysis.Analyzer{
	Name:     "nilchansend",
	Doc:      "reports a send or receive on a channel that is nil at that point",
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	info := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	for _, fn := range sourceFuncs(info) {
		f := flowFacts(fn)
		for _, block := range fn.Blocks {
			facts := f.isNil[block.Index]
			for _, instr := range block.Instrs {
				switch instr := instr.(type) {
				case *ssa.Send:
					if f.knownNil(instr.Chan, facts, f.nonNil[block.Index]) {
						pass.Reportf(instr.Pos(), sendMessage)
					}
				case *ssa.UnOp:
					if instr.Op != token.ARROW {
						continue
					}
					if f.knownNil(instr.X, facts, f.nonNil[block.Index]) {
						pass.Reportf(instr.Pos(), receiveMessage)
					}
				}
			}
		}
	}
	return nil, nil
}

// sourceFuncs returns the source functions and the package initializer.
func sourceFuncs(info *buildssa.SSA) []*ssa.Function {
	funcs := slices.Clone(info.SrcFuncs)
	if init, ok := info.Pkg.Members["init"].(*ssa.Function); ok {
		funcs = appendAnons(funcs, init)
	}
	return funcs
}

// appendAnons appends fn and its nested function literals.
func appendAnons(funcs []*ssa.Function, fn *ssa.Function) []*ssa.Function {
	funcs = append(funcs, fn)
	for _, anon := range fn.AnonFuncs {
		funcs = appendAnons(funcs, anon)
	}
	return funcs
}

// flow contains the nil and non-nil facts one function's blocks share.
type flow struct {
	isNil  []facts
	nonNil []facts
}

// facts contains values a branch has shown to be nil or non-nil.
type facts map[ssa.Value]struct{}

// edgeFunc returns the facts on one edge of the control-flow graph.
type edgeFunc func(pred *ssa.BasicBlock, in []facts, succ *ssa.BasicBlock) facts

// flowFacts returns the facts at the start of each block in fn.
func flowFacts(fn *ssa.Function) *flow {
	f := new(flow)
	f.nonNil = mustFacts(fn, nonNilEdge)
	f.isNil = mustFacts(fn, nilEdgeWith(f.nonNil))
	return f
}

// mustFacts returns the facts at the start of each block in fn.
func mustFacts(fn *ssa.Function, edgeOf edgeFunc) []facts {
	if len(fn.Blocks) == 0 {
		return nil
	}
	var (
		top  = guardValues(fn)
		live = liveBlocks(fn)
		in   = make([]facts, len(fn.Blocks))
	)
	for i, block := range fn.Blocks {
		if block != fn.Blocks[0] && live[block] {
			in[i] = clone(top)
			continue
		}
		in[i] = facts{}
	}
	for changed := true; changed; {
		changed = false
		for _, block := range fn.Blocks {
			if block == fn.Blocks[0] || !live[block] {
				continue
			}
			var (
				merged facts
				first  = true
			)
			for _, pred := range block.Preds {
				if !live[pred] {
					continue
				}
				edge := edgeOf(pred, in, block)
				if first {
					merged = edge
					first = false
					continue
				}
				keep(merged, edge)
			}
			if first {
				merged = facts{}
			}
			if same(in[block.Index], merged) {
				continue
			}
			in[block.Index] = merged
			changed = true
		}
	}
	return in
}

// guardValues returns the values a nil comparison guards anywhere in fn.
func guardValues(fn *ssa.Function) facts {
	out := facts{}
	for _, block := range fn.Blocks {
		if len(block.Instrs) == 0 {
			continue
		}
		cond, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
		if !ok {
			continue
		}
		if value, _ := nilCondition(cond.Cond); value != nil {
			out[value] = struct{}{}
		}
	}
	return out
}

// liveBlocks returns the blocks reachable from the entry block.
func liveBlocks(fn *ssa.Function) map[*ssa.BasicBlock]bool {
	var (
		live  = map[*ssa.BasicBlock]bool{fn.Blocks[0]: true}
		queue = []*ssa.BasicBlock{fn.Blocks[0]}
	)
	for len(queue) > 0 {
		block := queue[0]
		queue = queue[1:]
		for _, succ := range block.Succs {
			if live[succ] {
				continue
			}
			live[succ] = true
			queue = append(queue, succ)
		}
	}
	return live
}

// nilEdgeWith returns a nil-edge function that knows the non-nil facts.
func nilEdgeWith(nonNil []facts) edgeFunc {
	return func(pred *ssa.BasicBlock, in []facts, succ *ssa.BasicBlock) facts {
		return nilEdge(pred, in, nonNil, succ)
	}
}

// nilEdge returns the nil facts on the edge from pred to succ.
func nilEdge(pred *ssa.BasicBlock, in, nonNil []facts, succ *ssa.BasicBlock) facts {
	var (
		out                    = clone(in[pred.Index])
		value, nilWhenTrue, ok = edgeCondition(pred)
	)
	if !ok {
		return out
	}
	if knownNonNil(nonNil, value, nonNilEdge(pred, nonNil, succ)) {
		delete(out, value)
		return out
	}
	if (succ == pred.Succs[0]) == nilWhenTrue {
		out[value] = struct{}{}
	}
	return out
}

// nonNilEdge returns the non-nil facts on the edge from pred to succ.
func nonNilEdge(pred *ssa.BasicBlock, in []facts, succ *ssa.BasicBlock) facts {
	var (
		out                    = clone(in[pred.Index])
		value, nilWhenTrue, ok = edgeCondition(pred)
	)
	if !ok || (succ == pred.Succs[0]) == nilWhenTrue {
		return out
	}
	out[value] = struct{}{}
	return out
}

// edgeCondition returns the value a branch compares with nil.
func edgeCondition(pred *ssa.BasicBlock) (value ssa.Value, nilWhenTrue, ok bool) {
	if len(pred.Succs) != 2 || len(pred.Instrs) == 0 {
		return nil, false, false
	}
	cond, ok := pred.Instrs[len(pred.Instrs)-1].(*ssa.If)
	if !ok {
		return nil, false, false
	}
	value, nilWhenTrue = nilCondition(cond.Cond)
	return value, nilWhenTrue, value != nil
}

// nilCondition returns the value cond compares with nil and whether cond being
// true means that value is nil.
func nilCondition(cond ssa.Value) (ssa.Value, bool) {
	switch cond := cond.(type) {
	case *ssa.UnOp:
		if cond.Op == token.NOT {
			value, nilWhenTrue := nilCondition(cond.X)
			return value, !nilWhenTrue
		}
	case *ssa.BinOp:
		if cond.Op != token.EQL && cond.Op != token.NEQ {
			return nil, false
		}
		var value ssa.Value
		switch {
		case nilConst(cond.X):
			value = cond.Y
		case nilConst(cond.Y):
			value = cond.X
		default:
			return nil, false
		}
		if value == nil {
			return nil, false
		}
		return value, cond.Op == token.EQL
	}
	return nil, false
}

// nilConst reports whether v is the predeclared nil.
func nilConst(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	return ok && c.Value == nil
}

// knownNil reports whether v is nil on every path that reaches it.
func (f *flow) knownNil(v ssa.Value, at, nonNilAt facts) bool {
	return f.nilness(v, at, nonNilAt, make(map[ssa.Value]bool))
}

// nilness is knownNil with a recursion stack for phi cycles.
func (f *flow) nilness(v ssa.Value, at, nonNilAt facts, visiting map[ssa.Value]bool) bool {
	if _, ok := nonNilAt[v]; ok {
		return false
	}
	if _, ok := at[v]; ok {
		return true
	}
	if visiting[v] {
		return false
	}
	visiting[v] = true
	defer delete(visiting, v)
	switch v := v.(type) {
	case *ssa.Const:
		return v.Value == nil
	case *ssa.Phi:
		if len(v.Edges) == 0 {
			return false
		}
		block := v.Block()
		for i, edge := range v.Edges {
			if edge == v {
				continue
			}
			var (
				pred    = block.Preds[i]
				facts   = nilEdge(pred, f.isNil, f.nonNil, block)
				nonNils = nonNilEdge(pred, f.nonNil, block)
			)
			if !f.nilness(edge, facts, nonNils, visiting) {
				return false
			}
		}
		return true
	case *ssa.Convert:
		return f.nilness(v.X, at, nonNilAt, visiting)
	case *ssa.ChangeType:
		return f.nilness(v.X, at, nonNilAt, visiting)
	case *ssa.ChangeInterface:
		return f.nilness(v.X, at, nonNilAt, visiting)
	}
	return false
}

// knownNonNil reports whether v is non-nil from a make, a non-nil fact, a
// conversion, or a merge whose incoming values are all non-nil.
func knownNonNil(nonNil []facts, v ssa.Value, at facts) bool {
	return nonNilness(nonNil, v, at, make(map[ssa.Value]bool))
}

// nonNilness is knownNonNil with a recursion stack for phi cycles.
func nonNilness(nonNil []facts, v ssa.Value, at facts, visiting map[ssa.Value]bool) bool {
	if _, ok := at[v]; ok {
		return true
	}
	if visiting[v] {
		return true
	}
	visiting[v] = true
	defer delete(visiting, v)
	switch v := v.(type) {
	case *ssa.MakeChan:
		return true
	case *ssa.Phi:
		if len(v.Edges) == 0 {
			return false
		}
		block := v.Block()
		for i, edge := range v.Edges {
			var (
				pred  = block.Preds[i]
				facts = nonNilEdge(pred, nonNil, block)
			)
			if !nonNilness(nonNil, edge, facts, visiting) {
				return false
			}
		}
		return true
	case *ssa.Convert:
		return nonNilness(nonNil, v.X, at, visiting)
	case *ssa.ChangeType:
		return nonNilness(nonNil, v.X, at, visiting)
	case *ssa.ChangeInterface:
		return nonNilness(nonNil, v.X, at, visiting)
	}
	return false
}

// keep drops the facts in out that other does not hold.
func keep(out, other facts) {
	for value := range out {
		if _, ok := other[value]; !ok {
			delete(out, value)
		}
	}
}

// clone returns a copy of f.
func clone(f facts) facts {
	out := make(facts, len(f))
	for value := range f {
		out[value] = struct{}{}
	}
	return out
}

// same reports whether x and y hold the same facts.
func same(x, y facts) bool {
	if len(x) != len(y) {
		return false
	}
	for value := range x {
		if _, ok := y[value]; !ok {
			return false
		}
	}
	return true
}
