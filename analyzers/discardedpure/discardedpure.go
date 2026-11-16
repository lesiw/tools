// Package discardedpure provides an analysis.Analyzer that reports a call
// whose only effect is its result, written as a statement.
//
// The "Expression statements" section of the Go spec allows a function or
// method call in statement context
// (https://go.dev/ref/spec#Expression_statements). The call computes a result
// and the result is discarded. A function with no other effect changes nothing
// else, so the statement does nothing.
//
// The body of a function in the package under analysis is checked for effects:
// a write outside a value the function allocates, a map write, a send, a
// receive, a panic, a go statement, and a call that is not itself free of
// effects. A function from another package is recognized from a list of
// standard library functions whose effect is their result. A function with no
// results, a function whose parameter or receiver can hold a reference, and a
// recursive function are not judged. A call through a function literal, a
// function value, or an interface is left alone. The README catalogs every
// check and exemption.
//
// A call in a test file, and a call in a function with a *testing.B parameter,
// are left alone: a test or benchmark may discard a result to exercise the
// call.
//
// Only a call instruction is reported, so a deferred or go call is left alone.
// A result assigned to the blank identifier is reported, since the assignment
// discards it too.
//
// Report only: the caller may have meant to assign the result, and either the
// assignment or the removal is a plausible repair.
//
// The rule is adapted from staticcheck's SA4017.
package discardedpure

import (
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:     "discardedpure",
	Doc:      "reports a call whose result is discarded and has no other effect",
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	var (
		info = pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
		p    = make(purity)
	)
	for _, fn := range info.SrcFuncs {
		if testFile(pass, fn) || benchmark(fn) {
			continue
		}
		reach := reachable(fn)
		for _, block := range fn.Blocks {
			if !reach[block] {
				continue
			}
			for _, instr := range block.Instrs {
				call, ok := instr.(*ssa.Call)
				if !ok || !discarded(call) {
					continue
				}
				callee := call.Common().StaticCallee()
				if callee == nil || !p.pure(callee) {
					continue
				}
				name := callee.Name()
				if obj, ok := callee.Object().(*types.Func); ok {
					name = obj.FullName()
				}
				msg := "result of " + name + " is discarded; the call has no other effect"
				pass.Report(analysis.Diagnostic{Pos: call.Pos(), Message: msg})
			}
		}
	}
	return nil, nil
}

// discarded reports whether nothing reads the result of call. A debug
// reference records a name for a value but does not use it.
func discarded(call *ssa.Call) bool {
	refs := call.Referrers()
	if refs == nil {
		return true
	}
	for _, ref := range *refs {
		if _, ok := ref.(*ssa.DebugRef); !ok {
			return false
		}
	}
	return true
}

// testFile reports whether fn is written in a test file, which is a fixture
// for the package.
func testFile(pass *analysis.Pass, fn *ssa.Function) bool {
	return strings.HasSuffix(pass.Fset.Position(fn.Pos()).Filename, "_test.go")
}

// benchmark reports whether fn takes a *testing.B, which a helper called from
// a benchmark may do too.
func benchmark(fn *ssa.Function) bool {
	for param := range fn.Signature.Params().Variables() {
		ptr, ok := types.Unalias(param.Type()).(*types.Pointer)
		if !ok {
			continue
		}
		named, ok := types.Unalias(ptr.Elem()).(*types.Named)
		if !ok || named.Obj().Pkg() == nil {
			continue
		}
		pkg := named.Obj().Pkg()
		if pkg.Path() == "testing" && named.Obj().Name() == "B" {
			return true
		}
	}
	return false
}

// reachable returns the blocks reachable from the entry block. An if whose
// condition is a constant follows one successor, so a branch the compiler
// leaves out is not reached.
func reachable(fn *ssa.Function) map[*ssa.BasicBlock]bool {
	var (
		out  = make(map[*ssa.BasicBlock]bool)
		walk func(*ssa.BasicBlock)
	)
	walk = func(block *ssa.BasicBlock) {
		if block == nil || out[block] {
			return
		}
		out[block] = true
		if n := len(block.Instrs); n > 0 {
			if branch, ok := block.Instrs[n-1].(*ssa.If); ok {
				if c, ok := branch.Cond.(*ssa.Const); ok && c.Value != nil {
					if constant.BoolVal(c.Value) {
						walk(block.Succs[0])
					} else {
						walk(block.Succs[1])
					}
					return
				}
			}
		}
		for _, succ := range block.Succs {
			walk(succ)
		}
	}
	if len(fn.Blocks) > 0 {
		walk(fn.Blocks[0])
	}
	return out
}

// state is what the purity check has decided about a function.
type state uint8

const (
	unvisited state = iota
	visiting
	pureState
	impureState
)

// purity caches the verdict for each function it visits.
type purity map[*ssa.Function]state

// pure reports whether fn has no effect other than its result.
func (p purity) pure(fn *ssa.Function) bool {
	if fn == nil {
		return false
	}
	switch p[fn] {
	case pureState:
		return true
	case visiting, impureState:
		return false
	}
	obj, ok := fn.Object().(*types.Func)
	if !ok {
		p[fn] = impureState
		return false
	}
	if _, ok := pureStdlib[obj.FullName()]; ok {
		p[fn] = pureState
		return true
	}
	if !checkable(fn) {
		p[fn] = impureState
		return false
	}
	// A function that reaches itself returns false while it is being
	// visited, which leaves the recursion unproven.
	p[fn] = visiting
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if !p.effectFree(instr) {
				p[fn] = impureState
				return false
			}
		}
	}
	p[fn] = pureState
	return true
}

// checkable reports whether fn has a body, a result, and only basic
// parameters and receivers.
func checkable(fn *ssa.Function) bool {
	if fn.Blocks == nil || fn.Signature.Results().Len() == 0 {
		return false
	}
	return basicParams(fn)
}

// basicParams reports whether every parameter and receiver is a basic value,
// or a struct of basic values. A value that can hold a reference is left
// alone, since a write through the reference is not always visible in the
// body.
func basicParams(fn *ssa.Function) bool {
	for _, param := range fn.Params {
		if !basic(param.Type()) {
			return false
		}
	}
	return true
}

// basic reports whether t is a basic value, or a struct of basic values.
func basic(t types.Type) bool {
	switch t := types.Unalias(t).Underlying().(type) {
	case *types.Basic:
		return t.Kind() != types.UnsafePointer
	case *types.Struct:
		for field := range t.Fields() {
			if !basic(field.Type()) {
				return false
			}
		}
		return true
	}
	return false
}

// effectFree reports whether instr leaves the program unchanged, apart from
// values the function itself allocated.
func (p purity) effectFree(instr ssa.Instruction) bool {
	switch instr := instr.(type) {
	case *ssa.Call:
		return p.effectFreeCall(instr.Common())
	case *ssa.Defer:
		return p.effectFreeCall(&instr.Call)
	case *ssa.UnOp:
		return instr.Op != token.ARROW
	case *ssa.Store:
		return allocated(instr.Addr)
	case *ssa.MapUpdate, *ssa.Send, *ssa.Go, *ssa.Select, *ssa.Panic:
		return false
	}
	return true
}

// effectFreeCall reports whether a call changes nothing but the result.
func (p purity) effectFreeCall(call *ssa.CallCommon) bool {
	if call.IsInvoke() {
		return false
	}
	if builtin, ok := call.Value.(*ssa.Builtin); ok {
		switch builtin.Name() {
		case "cap", "complex", "imag", "len", "max", "min", "real":
			return true
		}
		return false
	}
	callee := call.StaticCallee()
	return callee != nil && p.pure(callee)
}

// allocated reports whether addr points into a value the function allocated.
func allocated(addr ssa.Value) bool {
	switch addr := addr.(type) {
	case *ssa.Alloc:
		return true
	case *ssa.FieldAddr:
		return allocated(addr.X)
	case *ssa.IndexAddr:
		return allocated(addr.X)
	case *ssa.Slice:
		return allocated(addr.X)
	}
	return false
}

// pureStdlib lists standard library functions whose effect is their result.
// The list matches the one staticcheck uses for SA4017.
var pureStdlib = map[string]struct{}{
	"errors.New":                      {},
	"fmt.Errorf":                      {},
	"fmt.Sprint":                      {},
	"fmt.Sprintf":                     {},
	"sort.Reverse":                    {},
	"strings.Map":                     {},
	"strings.Repeat":                  {},
	"strings.Replace":                 {},
	"strings.Title":                   {},
	"strings.ToLower":                 {},
	"strings.ToLowerSpecial":          {},
	"strings.ToTitle":                 {},
	"strings.ToTitleSpecial":          {},
	"strings.ToUpper":                 {},
	"strings.ToUpperSpecial":          {},
	"strings.Trim":                    {},
	"strings.TrimFunc":                {},
	"strings.TrimLeft":                {},
	"strings.TrimLeftFunc":            {},
	"strings.TrimPrefix":              {},
	"strings.TrimRight":               {},
	"strings.TrimRightFunc":           {},
	"strings.TrimSpace":               {},
	"strings.TrimSuffix":              {},
	"(*net/http.Request).WithContext": {},
	"time.Now":                        {},
	"time.Parse":                      {},
	"time.ParseInLocation":            {},
	"time.Unix":                       {},
	"time.UnixMicro":                  {},
	"time.UnixMilli":                  {},
	"(time.Time).Add":                 {},
	"(time.Time).AddDate":             {},
	"(time.Time).After":               {},
	"(time.Time).Before":              {},
	"(time.Time).Clock":               {},
	"(time.Time).Compare":             {},
	"(time.Time).Date":                {},
	"(time.Time).Day":                 {},
	"(time.Time).Equal":               {},
	"(time.Time).Format":              {},
	"(time.Time).GoString":            {},
	"(time.Time).GobEncode":           {},
	"(time.Time).Hour":                {},
	"(time.Time).ISOWeek":             {},
	"(time.Time).In":                  {},
	"(time.Time).IsDST":               {},
	"(time.Time).IsZero":              {},
	"(time.Time).Local":               {},
	"(time.Time).Location":            {},
	"(time.Time).MarshalBinary":       {},
	"(time.Time).MarshalJSON":         {},
	"(time.Time).MarshalText":         {},
	"(time.Time).Minute":              {},
	"(time.Time).Month":               {},
	"(time.Time).Nanosecond":          {},
	"(time.Time).Round":               {},
	"(time.Time).Second":              {},
	"(time.Time).String":              {},
	"(time.Time).Sub":                 {},
	"(time.Time).Truncate":            {},
	"(time.Time).UTC":                 {},
	"(time.Time).Unix":                {},
	"(time.Time).UnixMicro":           {},
	"(time.Time).UnixMilli":           {},
	"(time.Time).UnixNano":            {},
	"(time.Time).Weekday":             {},
	"(time.Time).Year":                {},
	"(time.Time).YearDay":             {},
	"(time.Time).Zone":                {},
	"(time.Time).ZoneBounds":          {},
}
