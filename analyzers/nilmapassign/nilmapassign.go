// Package nilmapassign provides an analysis.Analyzer that reports an
// assignment to an element of a nil map.
//
// The specification's "Index expressions" section says that assigning to an
// element of a nil map causes a run-time panic
// (https://go.dev/ref/spec#Index_expressions), and "Map types" says the value
// of an uninitialized map is nil (https://go.dev/ref/spec#Map_types).
//
// The map has to be made before the write, and the analyzer cannot choose
// where: a literal at the declaration, a make call before a loop, and a field
// set by the caller are all repairs of a different shape. The rule reports
// without a fix.
//
// The rule is adapted from staticcheck's SA5000.
package nilmapassign

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:     "nilmapassign",
	Doc:      "reports an assignment to an element of a nil map",
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	info := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	for _, fn := range info.SrcFuncs {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				update, ok := instr.(*ssa.MapUpdate)
				if ok && nilMap(update.Map) {
					pass.Reportf(update.Pos(), "assignment to nil map")
				}
			}
		}
	}
	return nil, nil
}

// nilMap reports whether v holds the nil map on every path that reaches it.
func nilMap(v ssa.Value) bool {
	var (
		seen = make(map[ssa.Value]struct{})
		walk func(ssa.Value) bool
	)
	walk = func(v ssa.Value) bool {
		if _, ok := seen[v]; ok {
			return true
		}
		seen[v] = struct{}{}
		if phi, ok := v.(*ssa.Phi); ok {
			for _, edge := range phi.Edges {
				if !walk(edge) {
					return false
				}
			}
			return true
		}
		c, ok := v.(*ssa.Const)
		return ok && c.Value == nil
	}
	return walk(v)
}
