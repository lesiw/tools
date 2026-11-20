// Package yield provides an analysis.Analyzer that reports a call to a yield
// function that may be reached after an earlier call returned false.
//
// Vendored from gopls' yield (gopls/internal/analysis/yield,
// golang.org/x/tools/gopls v0.24.0, BSD-3-Clause, Copyright 2026 The Go
// Authors), which is internal and cannot be imported. The rule and the
// analysis are upstream's; the code is adapted to this repository's helpers,
// writing the lattice as a sorted list of conditions where gopls keeps a
// bitset and the fixpoint as a worklist where gopls uses its flow package. The
// fix is written here, because gopls reports only.
//
// The spec is the rule: a loop body that terminates makes yield return false,
// and the spec says it "must not be called again"
// (https://go.dev/ref/spec#For_range).
//
// The check is a may analysis: a call it cannot rule out is reported, so a
// condition it cannot see costs a false report rather than a missed one.
//
// The README names the calls the check counts, the conditions it cannot see,
// and the pairs it will not fix.
package yield

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name:     "yield",
	Doc:      "reports a call to a yield function reached after an earlier one returned false",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	if !importsIter(pass.Pkg) {
		return nil, nil
	}
	found := findYields(pass)
	if len(found.calls) == 0 {
		return nil, nil
	}
	result := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	for _, fn := range result.SrcFuncs {
		check(pass, found, fn)
	}
	return nil, nil
}

// importsIter reports whether pkg imports iter, which stands in for the
// package defining iterators. It is not strictly necessary, and gopls uses the
// same proxy.
func importsIter(pkg *types.Package) bool {
	for _, imp := range pkg.Imports() {
		if imp.Path() == "iter" {
			return true
		}
	}
	return false
}

// yields holds the yield calls of a package.
type yields struct {
	// calls is keyed by the position of the call's opening parenthesis, which
	// is the position the instruction reports.
	calls map[token.Pos]*ast.CallExpr

	// next is the yield call that the next statement makes, when the call
	// holds a statement of its own and so does the one after it.
	next map[*ast.CallExpr]*ast.CallExpr
}

// findYields collects the yield calls of the package.
func findYields(pass *analysis.Pass) yields {
	out := yields{
		calls: make(map[token.Pos]*ast.CallExpr),
		next:  make(map[*ast.CallExpr]*ast.CallExpr),
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if isYield(pass, n) {
					out.calls[n.Lparen] = n
				}
			case *ast.BlockStmt:
				out.link(pass, n.List)
			case *ast.CaseClause:
				out.link(pass, n.Body)
			case *ast.CommClause:
				out.link(pass, n.Body)
			}
			return true
		})
	}
	return out
}

// link records each yield call that holds one of list's statements and the
// call that holds the statement after it.
func (y yields) link(pass *analysis.Pass, list []ast.Stmt) {
	var before *ast.CallExpr
	for _, stmt := range list {
		call := stmtCall(pass, stmt)
		if before != nil && call != nil {
			y.next[before] = call
		}
		before = call
	}
}

// stmtCall returns the call that stmt is, when stmt holds nothing but a yield
// call.
func stmtCall(pass *analysis.Pass, stmt ast.Stmt) *ast.CallExpr {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := ast.Unparen(expr.X).(*ast.CallExpr)
	if !ok || !isYield(pass, call) {
		return nil
	}
	return call
}

// isYield reports whether call invokes a function named yield with at most two
// parameters and one bool result.
func isYield(pass *analysis.Pass, call *ast.CallExpr) bool {
	id, ok := call.Fun.(*ast.Ident)
	if !ok || id.Name != "yield" {
		return false
	}
	sig, ok := pass.TypesInfo.TypeOf(id).(*types.Signature)
	if !ok || sig.Params().Len() >= 3 || sig.Results().Len() != 1 {
		return false
	}
	return types.Identical(sig.Results().At(0).Type(), types.Typ[types.Bool])
}

// checker holds the state of one function's analysis.
type checker struct {
	pass   *analysis.Pass
	yields yields
	blocks []*ssa.BasicBlock
	numb   numbering
}

// check reports the yield calls in fn that may run after a call that
// returned false.
func check(pass *analysis.Pass, found yields, fn *ssa.Function) {
	c := checker{
		pass:   pass,
		yields: found,
		blocks: fn.Blocks,
		numb:   make(numbering),
	}
	if !c.hasCall() {
		return
	}
	c.report(solve(c.blocks, c.transfer))
}

// hasCall reports whether the function makes a yield call.
func (c *checker) hasCall() bool {
	return slices.ContainsFunc(c.blocks, func(block *ssa.BasicBlock) bool {
		return slices.ContainsFunc(block.Instrs, c.isCall)
	})
}

// isCall reports whether instr is the SSA instruction of a yield call.
func (c *checker) isCall(instr ssa.Instruction) bool {
	call, ok := instr.(*ssa.Call)
	return ok && c.yields.calls[call.Pos()] != nil
}

// transfer returns what one edge carries: the effect of from's instructions,
// then of the phis in to that this edge feeds.
func (c *checker) transfer(from, to int, in stateSet) stateSet {
	var (
		block = c.blocks[from]
		out   = in
	)
	for _, instr := range block.Instrs {
		switch instr := instr.(type) {
		case *ssa.Call:
			if c.isCall(instr) {
				out = out.yieldCall(c.numb, instr)
			}
		case *ssa.UnOp:
			if instr.Op == token.NOT {
				out = out.not(c.numb, instr)
			}
		case *ssa.If:
			out = out.branch(c.numb, instr.Cond, to == block.Succs[0].Index)
		}
	}
	if edge := slices.Index(c.blocks[to].Preds, block); edge >= 0 {
		for _, instr := range c.blocks[to].Instrs {
			if phi, ok := instr.(*ssa.Phi); ok {
				out = out.phi(c.numb, phi, phi.Edges[edge])
			}
		}
	}
	if len(out) > 0 && !sameSlice(out, in) {
		out = normalize(out)
	}
	return out
}

// problem is a call that returned false and a call it reaches afterward.
type problem struct{ first, later *ssa.Call }

// problems returns a pair for each path on which a call that may have returned
// false reaches another call, in source order.
func (c *checker) problems(ins []stateSet) (out []problem) {
	for i, block := range c.blocks {
		seen := ins[i]
		for _, instr := range block.Instrs {
			if !c.isCall(instr) {
				continue
			}
			call := instr.(*ssa.Call)
			for _, s := range seen {
				out = append(out, problem{s.yield, call})
			}
			seen = seen.yieldCall(c.numb, call)
		}
	}
	// Block order is not source order, and each call reports its earliest
	// later call.
	slices.SortFunc(out, func(x, y problem) int {
		if d := cmp.Compare(x.first.Pos(), y.first.Pos()); d != 0 {
			return d
		}
		return cmp.Compare(x.later.Pos(), y.later.Pos())
	})
	return out
}

// report names the calls that may run after a call that returned false. One
// fix repairs a whole run of calls, and it hangs off the pair at the head of
// the run, so the pairs inside it are reported without one.
func (c *checker) report(ins []stateSet) {
	var (
		problems = c.problems(ins)
		covered  = make(map[*ast.CallExpr]struct{})
		last     *ssa.Call
	)
	for _, p := range problems {
		if p.first == last {
			continue // this call already reported its earliest later call
		}
		last = p.first
		c.emit(p, covered)
	}
}

// emit reports one pair, with a fix when the pair heads the run of calls that
// the fix repairs. A pair of a call with itself takes no fix: that is how a
// loop body reports, since the call reaches itself through the back edge.
func (c *checker) emit(p problem, covered map[*ast.CallExpr]struct{}) {
	var (
		first   = c.yields.calls[p.first.Pos()]
		message = "yield may be called again after returning false"
		fixes   []analysis.SuggestedFix
	)
	if p.later != p.first {
		message = fmt.Sprintf(
			"yield may be called again (on L%d) after returning false",
			c.pass.Fset.PositionFor(p.later.Pos(), false).Line,
		)
		fixes = c.repair(p, covered)
	}
	c.pass.Report(analysis.Diagnostic{
		Pos:            first.Pos(),
		End:            first.End(),
		Message:        message,
		SuggestedFixes: fixes,
	})
}

// repair returns the fix that nests the run of calls this pair heads, and
// records the calls that fix now covers. A pair inside a covered run takes no
// fix, and neither does a call whose callee is not the enclosing function's
// own parameter: a package-level function, a local, or a captured parameter is
// reported on the strength of its name, and an edit that assumed the iterator
// protocol would change the program.
func (c *checker) repair(p problem, covered map[*ast.CallExpr]struct{}) []analysis.SuggestedFix {
	var (
		first = c.yields.calls[p.first.Pos()]
		later = c.yields.calls[p.later.Pos()]
	)
	if _, ok := covered[first]; ok {
		return nil
	}
	if _, ok := p.first.Common().Value.(*ssa.Parameter); !ok {
		return nil
	}
	chain := c.chain(first)
	if len(chain) < 2 || chain[1] != later {
		return nil
	}
	edits, ok := c.guard(chain)
	if !ok {
		return nil
	}
	for _, call := range chain {
		covered[call] = struct{}{}
	}
	return []analysis.SuggestedFix{{
		Message:   "Guard the later calls",
		TextEdits: edits,
	}}
}

// chain returns the calls that the fix nests: first, and each call that holds
// the statement after the one before it.
func (c *checker) chain(first *ast.CallExpr) []*ast.CallExpr {
	out := []*ast.CallExpr{first}
	for next := c.yields.next[first]; next != nil; next = c.yields.next[next] {
		out = append(out, next)
	}
	return out
}

// guard returns the edit that nests each call of chain behind the one before
// it, so that a call runs only when every call before it returned true. Each
// call must be alone on its lines, and each must be the statement before the
// next, so that the edit moves no other text.
func (c *checker) guard(chain []*ast.CallExpr) ([]analysis.TextEdit, bool) {
	var (
		first = chain[0]
		last  = chain[len(chain)-1]
		file  = c.pass.Fset.File(first.Pos())
	)
	// A file that changed since it was parsed would make these offsets point
	// at other text, so no fix is offered for it.
	src, err := c.pass.ReadFile(file.Name())
	if err != nil || len(src) != file.Size() {
		return nil, false
	}
	start := file.Offset(first.Pos())
	indent := string(src[source.Start(src, start):start])
	// The edit writes tabs, so it is only offered on code indented with them.
	if strings.TrimLeft(indent, "\t") != "" {
		return nil, false
	}
	texts := make([]string, len(chain))
	for i, call := range chain {
		lo, hi := file.Offset(call.Pos()), file.Offset(call.End())
		// Anything else on a call's line, a comment or another statement,
		// would be moved or lost.
		if !source.Alone(src, lo, hi) {
			return nil, false
		}
		// A call written across lines would keep the indentation the edit
		// gives the line it starts on, so it is left alone.
		texts[i] = string(src[lo:hi])
		if strings.Contains(texts[i], "\n") {
			return nil, false
		}
	}
	// A comment or a blank line between two calls would be deleted.
	var nl string
	for i := 1; i < len(chain); i++ {
		var ok bool
		if nl, ok = gap(file, src, chain[i-1], chain[i]); !ok {
			return nil, false
		}
	}
	var out strings.Builder
	for i, text := range texts {
		if i > 0 {
			out.WriteString(nl)
			out.WriteString(indent)
			out.WriteString(strings.Repeat("\t", i))
		}
		if i == len(texts)-1 {
			out.WriteString(text)
			continue
		}
		out.WriteString("if ")
		out.WriteString(text)
		out.WriteString(" {")
	}
	for i := len(texts) - 2; i >= 0; i-- {
		out.WriteString(nl)
		out.WriteString(indent)
		out.WriteString(strings.Repeat("\t", i))
		out.WriteString("}")
	}
	// The last line ends where its line break does, so whitespace after the
	// call goes with the edit rather than trailing the new brace.
	end := source.End(src, file.Offset(last.End())) - 1
	if end > 0 && src[end-1] == '\r' {
		end--
	}
	return []analysis.TextEdit{{
		Pos:     first.Pos(),
		End:     file.Pos(end),
		NewText: []byte(out.String()),
	}}, true
}

// gap returns the line break between two calls, when the text between them is
// that break and the indentation of the line after it, so that the edit
// deletes only whitespace.
func gap(file *token.File, src []byte, before, after *ast.CallExpr) (string, bool) {
	var (
		lo, hi  = file.Offset(before.End()), file.Offset(after.Pos())
		between = string(src[lo:hi])
		i       = strings.IndexByte(between, '\n')
		blank   = strings.TrimLeft(between, " \t\r\n") == ""
	)
	if i < 0 || strings.Count(between, "\n") != 1 || !blank {
		return "", false
	}
	if i > 0 && between[i-1] == '\r' {
		return between[i-1 : i+1], true
	}
	return between[i : i+1], true
}

// numbering maps the values a function mentions to small integers, so that a
// set of conditions has one sorted form.
type numbering map[ssa.Value]int

// number returns the number of v, assigning one if it has none.
func (n numbering) number(v ssa.Value) int {
	i, ok := n[v]
	if !ok {
		i = len(n)
		n[v] = i
	}
	return i
}

// state is a path on which a yield call returned false, restricted to the
// conditions in conds. The call's own condition is never dropped, so no state
// is empty.
type state struct {
	yield *ssa.Call // the call that returned false
	conds []cond    // the conditions, sorted by number
}

// cond is the truth of one local boolean value, numbered so that a set of them
// can be sorted.
type cond struct {
	num   int
	sense bool
}

// stateSet is the fact on one edge: every yield call that may have returned
// false on a path to that edge, with the conditions on the path.
type stateSet []state

// byNum compares a condition with a value's number.
func byNum(c cond, num int) int { return cmp.Compare(c.num, num) }

// search returns the condition recorded for the value numbered num.
func search(conds []cond, num int) (cond, bool) {
	i, found := slices.BinarySearchFunc(conds, num, byNum)
	if !found {
		return cond{}, false
	}
	return conds[i], true
}

// equal reports whether x and y say the same thing.
func (x state) equal(y state) bool {
	return x.yield == y.yield && slices.Equal(x.conds, y.conds)
}

// stricter reports whether x says everything y says, so that y adds nothing.
func (x state) stricter(y state) bool {
	if x.yield != y.yield {
		return false
	}
	for _, c := range y.conds {
		if got, ok := search(x.conds, c.num); !ok || got.sense != c.sense {
			return false
		}
	}
	return true
}

// set returns x with the condition on the value numbered num set to sense.
func (x state) set(num int, sense bool) state {
	x.conds = slices.Clone(x.conds)
	i, found := slices.BinarySearchFunc(x.conds, num, byNum)
	if !found {
		x.conds = slices.Insert(x.conds, i, cond{num, sense})
		return x
	}
	x.conds[i].sense = sense
	return x
}

// clear returns x without the condition on the value numbered num.
func (x state) clear(num int) state {
	x.conds = slices.Clone(x.conds)
	i, found := slices.BinarySearchFunc(x.conds, num, byNum)
	if !found {
		return x
	}
	x.conds = slices.Delete(x.conds, i, i+1)
	return x
}

// compareStates orders states so that a set of them has one sorted form.
func compareStates(x, y state) int {
	if x.yield != y.yield {
		return cmp.Compare(x.yield.Pos(), y.yield.Pos())
	}
	if d := cmp.Compare(len(x.conds), len(y.conds)); d != 0 {
		return d
	}
	for i, c := range x.conds {
		if d := cmp.Compare(c.num, y.conds[i].num); d != 0 {
			return d
		}
		if c.sense != y.conds[i].sense {
			if !c.sense {
				return -1
			}
			return 1
		}
	}
	return 0
}

// normalize returns the set in the form its states are compared in: sorted,
// with a state that a broader one covers removed. It sorts and truncates ss,
// so the caller must own it.
func normalize(ss stateSet) stateSet {
	slices.SortFunc(ss, compareStates)
	out := ss[:0]
	for _, s := range ss {
		if slices.ContainsFunc(out, s.stricter) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// sameSet reports whether two facts hold the same states.
func sameSet(a, b stateSet) bool {
	return slices.EqualFunc(a, b, state.equal)
}

// merge joins two facts. The result is normalized, and a new slice, so neither
// argument is written.
func merge(a, b stateSet) stateSet {
	return normalize(slices.Concat(a, b))
}

// sameSlice reports whether x and y share a backing array, so that normalizing
// one would write the other.
func sameSlice(x, y stateSet) bool {
	return len(x) == len(y) && (len(x) == 0 || &x[0] == &y[0])
}

// yieldCall returns in with a state added for a call that has just returned,
// which the analysis takes to have returned false.
func (in stateSet) yieldCall(n numbering, call *ssa.Call) stateSet {
	s := state{yield: call, conds: []cond{{n.number(call), false}}}
	return append(slices.Clone(in), s)
}

// not returns in with the condition on the negation's result, which holds when
// the operand holds and does not.
func (in stateSet) not(n numbering, unop *ssa.UnOp) (out stateSet) {
	var (
		xnum   = n.number(unop.X)
		notnum = n.number(unop)
	)
	for _, s := range in {
		if c, ok := search(s.conds, xnum); ok {
			out = append(out, s.set(notnum, !c.sense))
			continue
		}
		out = append(out, s.clear(notnum)) // a stale condition goes
	}
	return out
}

// phi returns in with the condition on the phi, which this edge gives the
// value val.
func (in stateSet) phi(n numbering, phi *ssa.Phi, val ssa.Value) (out stateSet) {
	phinum := n.number(phi)
	if sense, ok := boolean(val); ok {
		for _, s := range in {
			out = append(out, s.set(phinum, sense))
		}
		return out
	}
	valnum := n.number(val)
	for _, s := range in {
		if c, ok := search(s.conds, valnum); ok {
			out = append(out, s.set(phinum, c.sense))
			continue
		}
		out = append(out, s.clear(phinum))
	}
	return out
}

// boolean returns the value of an operand that is a constant boolean.
func boolean(v ssa.Value) (sense, ok bool) {
	c, ok := v.(*ssa.Const)
	if !ok || c.Value == nil || c.Value.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(c.Value), true
}

// branch returns in restricted to the paths that take the branch where cond
// has the given sense, dropping a state whose conditions make that branch
// impossible.
func (in stateSet) branch(n numbering, cond ssa.Value, sense bool) (out stateSet) {
	for {
		unop, ok := cond.(*ssa.UnOp)
		if !ok || unop.Op != token.NOT {
			break
		}
		sense = !sense
		cond = unop.X
	}
	num := n.number(cond)
	for _, s := range in {
		if c, ok := search(s.conds, num); ok && c.sense != sense {
			continue
		}
		out = append(out, s)
	}
	return out
}

// edgeRef names one edge out of a block by the block and the successor's place
// in its successor list.
type edgeRef struct {
	from, edge int
}

// solve returns the fact on entry to each block, the least fixpoint of the
// transfer function over the block graph. The transfer function is handed the
// block it leaves and the block it enters.
func solve(blocks []*ssa.BasicBlock, transfer func(from, to int, in stateSet) stateSet) []stateSet {
	var (
		ins     = make([]stateSet, len(blocks))
		outs    = make([][]stateSet, len(blocks))
		preds   = make([][]edgeRef, len(blocks))
		fresh   = make([]bool, len(blocks))
		queued  = make([]bool, len(blocks))
		queue   []int
		enqueue = func(i int) {
			if !queued[i] {
				queued[i] = true
				queue = append(queue, i)
			}
		}
	)
	for i, block := range blocks {
		outs[i] = make([]stateSet, len(block.Succs))
		fresh[i] = true
		for k, succ := range block.Succs {
			preds[succ.Index] = append(preds[succ.Index], edgeRef{i, k})
		}
		enqueue(i)
	}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		queued[i] = false
		// A block with no predecessor, the entry among them, holds no
		// state.
		var in stateSet
		for k, pred := range preds[i] {
			// The first fact is taken as it stands, so a block reached
			// one way shares its predecessor's.
			if k == 0 {
				in = outs[pred.from][pred.edge]
				continue
			}
			in = merge(in, outs[pred.from][pred.edge])
		}
		if !fresh[i] && sameSet(in, ins[i]) {
			continue
		}
		first := fresh[i]
		fresh[i], ins[i] = false, in
		for k, succ := range blocks[i].Succs {
			out := transfer(i, succ.Index, in)
			if !first && sameSet(out, outs[i][k]) {
				continue
			}
			outs[i][k] = out
			enqueue(succ.Index)
		}
	}
	return ins
}
