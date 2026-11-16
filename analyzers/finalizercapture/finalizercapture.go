// Package finalizercapture provides an analysis.Analyzer that reports a
// runtime.SetFinalizer call whose finalizer captures the object through its
// own variable or bound receiver.
//
// The runtime.SetFinalizer documentation describes the cycle: "If a cyclic
// structure includes a block with a finalizer, that cycle is not guaranteed to
// be garbage collected and the finalizer is not guaranteed to run"
// (https://pkg.go.dev/runtime#SetFinalizer). The object is reachable from the
// finalizer, so it may not be collected and the finalizer may not run.
//
// The rule reports a call whose second argument is a function literal or
// method value that captures the first argument. A function literal stored in
// a local variable is followed to the call. A method value is reported when it
// is written at the call, its receiver expression is the object, and the
// method has a pointer receiver. A stored method value, a promoted method, and
// a method value with a value receiver are left alone. A finalizer that
// reaches the object only through its parameter is not reported. A finalizer
// function literal that reads the object from a package variable is not
// reported, since the closure does not capture the object. A finalizer that
// reaches the object only through a captured struct or another local is left
// alone. The rule reports the capture at the call and does not analyze a later
// assignment
// to the object variable; the finalizer may read a different value by the time
// it runs, but the object is still referenced at registration. A finalizer
// obtained from a call is not followed.
//
// A fix rewrites a function literal to use its parameter in place of the
// captured object. The fix is offered only when the finalizer is a function
// literal written at the call. A method value has no literal body at the call
// to rewrite. A function literal stored in a variable may be used more than
// once, so the fix is withheld. The fix is also withheld when the object is
// not an identifier, when the parameter has a different type, when the
// finalizer captures another local, when the object is assigned or its address
// is taken anywhere, when the finalizer reassigns its parameter or takes its
// address, when a local declaration in the finalizer would shadow the
// parameter, or when a rewritten line would exceed the width. A fix is
// still offered when a rewritten line is already past the width.
//
// The rule is adapted from staticcheck's SA5005.
package finalizercapture

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name:     "finalizercapture",
	Doc:      "reports a finalizer that captures the object it finalizes",
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	captured := capturedCalls(pass)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if !captured[call.Lparen] {
				if !setFinalizer(pass, call) || !methodValue(pass, call) {
					return true
				}
			}
			report(pass, call)
			return true
		})
	}
	return nil, nil
}

func capturedCalls(pass *analysis.Pass) map[token.Pos]bool {
	var (
		captured = make(map[token.Pos]bool)
		info     = pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
		seen     = make(map[*ssa.Function]struct{})
		walk     func(*ssa.Function)
	)
	walk = func(fn *ssa.Function) {
		if fn == nil {
			return
		}
		if _, ok := seen[fn]; ok {
			return
		}
		seen[fn] = struct{}{}
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
				if captures(call, instr.Block()) {
					captured[call.Pos()] = true
				}
			}
		}
		for _, anon := range fn.AnonFuncs {
			walk(anon)
		}
	}
	for _, fn := range info.SrcFuncs {
		walk(fn)
	}
	for _, member := range info.Pkg.Members {
		if fn, ok := member.(*ssa.Function); ok {
			walk(fn)
		}
	}
	return captured
}

// A method value binds its receiver, so a $bound wrapper is a value reference;
// a function literal references the cell.
func captures(call *ssa.CallCommon, block *ssa.BasicBlock) bool {
	if len(call.Args) != 2 {
		return false
	}
	callee := call.StaticCallee()
	if callee == nil {
		return false
	}
	fn, ok := callee.Object().(*types.Func)
	if !ok || !setFinalizerFunc(fn) {
		return false
	}
	object, finalizer := cell(call.Args[0]), unwrapInterface(call.Args[1])
	for _, closure := range closures(finalizer, block, call.Pos()) {
		if strings.HasSuffix(closure.Fn.Name(), "$bound") {
			continue
		}
		if slices.Contains(closure.Bindings, object) {
			return true
		}
	}
	return false
}

// A finalizer variable forced into a cell is read at the call, so any value
// stored before the call may be the finalizer.
func closures(v ssa.Value, block *ssa.BasicBlock, before token.Pos) []*ssa.MakeClosure {
	var (
		out  []*ssa.MakeClosure
		seen = make(map[ssa.Value]struct{})
		walk func(ssa.Value)
	)
	walk = func(v ssa.Value) {
		if v == nil {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		switch v := v.(type) {
		case *ssa.MakeClosure:
			out = append(out, v)
		case *ssa.Phi:
			for _, edge := range v.Edges {
				walk(edge)
			}
		case *ssa.ChangeType:
			walk(v.X)
		case *ssa.ChangeInterface:
			walk(v.X)
		case *ssa.MakeInterface:
			walk(v.X)
		case *ssa.TypeAssert:
			walk(v.X)
		case *ssa.UnOp:
			if v.Op != token.MUL {
				return
			}
			alloc, ok := v.X.(*ssa.Alloc)
			if !ok {
				return
			}
			walkStores(alloc, block, before, walk)
		}
	}
	walk(v)
	return out
}

// A store overwritten on every path cannot be the value at the call.
func walkStores(alloc *ssa.Alloc, block *ssa.BasicBlock, before token.Pos, walk func(ssa.Value)) {
	var stores []*ssa.Store
	for _, ref := range *alloc.Referrers() {
		store, ok := ref.(*ssa.Store)
		if !ok || store.Addr != alloc {
			continue
		}
		stores = append(stores, store)
	}
	target := instructionAt(block, before)
	all := make(map[*ssa.Store]struct{})
	for _, store := range stores {
		all[store] = struct{}{}
	}
	for _, store := range stores {
		if target == nil && store.Pos() >= before {
			continue
		}
		if target != nil && !reaches(store, target, all) {
			continue
		}
		walk(store.Val)
	}
}

func instructionAt(block *ssa.BasicBlock, pos token.Pos) ssa.Instruction {
	for _, instr := range block.Instrs {
		if instr.Pos() == pos {
			return instr
		}
	}
	return nil
}

// state is one position in the control flow graph.
type state struct {
	block *ssa.BasicBlock
	index int
}

// A value reaches the call only on a path with no later store to the cell.
func reaches(store *ssa.Store, target ssa.Instruction, stores map[*ssa.Store]struct{}) bool {
	var (
		seen = make(map[state]struct{})
		walk func(*ssa.BasicBlock, int) bool
	)
	walk = func(block *ssa.BasicBlock, index int) bool {
		for block != nil && index < len(block.Instrs) {
			key := state{block: block, index: index}
			if _, ok := seen[key]; ok {
				return false
			}
			seen[key] = struct{}{}
			instr := block.Instrs[index]
			if instr == target {
				return true
			}
			if later, ok := instr.(*ssa.Store); ok {
				if _, ok := stores[later]; ok && later != store {
					return false
				}
			}
			index++
		}
		if block == nil {
			return false
		}
		for _, succ := range block.Succs {
			if walk(succ, 0) {
				return true
			}
		}
		return false
	}
	block := store.Block()
	var index int
	for i, instr := range block.Instrs {
		if instr == store {
			index = i + 1
			break
		}
	}
	return walk(block, index)
}

func setFinalizerFunc(fn *types.Func) bool {
	return fn.Pkg() != nil && fn.Pkg().Path() == "runtime" &&
		fn.Name() == "SetFinalizer" && fn.Signature().Recv() == nil
}

func unwrapInterface(v ssa.Value) ssa.Value {
	for {
		conversion, ok := v.(*ssa.MakeInterface)
		if !ok {
			return v
		}
		v = conversion.X
	}
}

// A captured local has one allocation, and both the object and the finalizer
// read it through that allocation.
func cell(v ssa.Value) ssa.Value {
	v = unwrapInterface(v)
	load, ok := v.(*ssa.UnOp)
	if !ok || load.Op != token.MUL {
		return v
	}
	alloc, ok := load.X.(*ssa.Alloc)
	if !ok {
		return v
	}
	return alloc
}

func setFinalizer(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 2 {
		return false
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	return ok && setFinalizerFunc(fn)
}

// The receiver must be the object expression; a promoted method binds the
// embedded field instead.
func methodValue(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := ast.Unparen(call.Args[1]).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	selection := pass.TypesInfo.Selections[sel]
	if selection == nil || selection.Kind() != types.MethodVal {
		return false
	}
	if len(selection.Index()) != 1 {
		return false
	}
	method, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	signature, ok := method.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return false
	}
	if _, ok := types.Unalias(signature.Recv().Type()).(*types.Pointer); !ok {
		return false
	}
	return astexpr.Same(pass, sel.X, call.Args[0])
}

func report(pass *analysis.Pass, call *ast.CallExpr) {
	diagnostic := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "finalizer captures the object it finalizes, so the object may not be collected",
	}
	if fix, ok := fix(pass, call); ok {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{fix}
	}
	pass.Report(diagnostic)
}

func fix(pass *analysis.Pass, call *ast.CallExpr) (analysis.SuggestedFix, bool) {
	fn, id, object, ok := target(pass, call)
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	if parameterMutated(pass, fn) || capturedOther(pass, fn, object) {
		return analysis.SuggestedFix{}, false
	}
	if mutatedObject(pass, object) {
		return analysis.SuggestedFix{}, false
	}
	name, edits, ok := rename(fn, id.Name)
	if !ok || defined(pass, fn.Body, name) {
		return analysis.SuggestedFix{}, false
	}
	replacements, ok := replace(pass, fn.Body, object, name)
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	edits = append(edits, replacements...)
	if !fits(pass, edits) {
		return analysis.SuggestedFix{}, false
	}
	return analysis.SuggestedFix{
		Message:   "Use the finalizer argument",
		TextEdits: edits,
	}, true
}

// literalAt returns the function literal in expr, through parentheses and
// conversions.
func literalAt(pass *analysis.Pass, expr ast.Expr) *ast.FuncLit {
	for {
		expr = ast.Unparen(expr)
		if fn, ok := expr.(*ast.FuncLit); ok {
			return fn
		}
		call, ok := expr.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return nil
		}
		if !pass.TypesInfo.Types[call.Fun].IsType() {
			return nil
		}
		expr = call.Args[0]
	}
}

func target(pass *analysis.Pass, call *ast.CallExpr) (*ast.FuncLit, *ast.Ident, types.Object, bool) {
	fn := literalAt(pass, call.Args[1])
	if fn == nil {
		return nil, nil, nil, false
	}
	id, ok := ast.Unparen(call.Args[0]).(*ast.Ident)
	if !ok {
		return nil, nil, nil, false
	}
	object := pass.TypesInfo.Uses[id]
	if object == nil {
		return nil, nil, nil, false
	}
	signature, ok := pass.TypesInfo.TypeOf(fn).(*types.Signature)
	if !ok || signature.Params().Len() != 1 {
		return nil, nil, nil, false
	}
	if !sameType(pass, call, signature) {
		return nil, nil, nil, false
	}
	return fn, id, object, true
}

func sameType(pass *analysis.Pass, call *ast.CallExpr, signature *types.Signature) bool {
	var (
		object = pass.TypesInfo.TypeOf(call.Args[0])
		param  = signature.Params().At(0).Type()
	)
	return types.Identical(object, param)
}

func rename(fn *ast.FuncLit, object string) (string, []analysis.TextEdit, bool) {
	param, name, ok := parameter(fn)
	if !ok {
		return "", nil, false
	}
	var edits []analysis.TextEdit
	switch name {
	case "":
		name = object
		edits = append(edits, analysis.TextEdit{
			Pos:     param.Type.Pos(),
			End:     param.Type.Pos(),
			NewText: []byte(name + " "),
		})
	case "_":
		name = object
		edits = append(edits, analysis.TextEdit{
			Pos:     param.Names[0].Pos(),
			End:     param.Names[0].End(),
			NewText: []byte(name),
		})
	}
	if name == "" {
		return "", nil, false
	}
	return name, edits, true
}

func parameter(fn *ast.FuncLit) (*ast.Field, string, bool) {
	if len(fn.Type.Params.List) != 1 {
		return nil, "", false
	}
	field := fn.Type.Params.List[0]
	switch len(field.Names) {
	case 0:
		return field, "", true
	case 1:
		return field, field.Names[0].Name, true
	}
	return nil, "", false
}

func replace(pass *analysis.Pass, body *ast.BlockStmt, object types.Object, name string) ([]analysis.TextEdit, bool) {
	var (
		edits []analysis.TextEdit
		stack []ast.Node
		found bool
		safe  = true
	)
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		ident, ok := n.(*ast.Ident)
		if ok && pass.TypesInfo.Uses[ident] == object {
			found = true
			if mutated(ident, stack) {
				safe = false
				return false
			}
			edits = append(edits, analysis.TextEdit{
				Pos:     ident.Pos(),
				End:     ident.End(),
				NewText: []byte(name),
			})
		}
		stack = append(stack, n)
		return true
	})
	return edits, safe && found
}

func defined(pass *analysis.Pass, body *ast.BlockStmt, name string) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		if ident, ok := n.(*ast.Ident); ok {
			object := pass.TypesInfo.Defs[ident]
			if object != nil && object.Name() == name {
				found = true
				return false
			}
		}
		implicit, ok := pass.TypesInfo.Implicits[n]
		if ok && implicit.Name() == name {
			found = true
			return false
		}
		return true
	})
	return found
}

func written(ident *ast.Ident, stack []ast.Node) bool {
	stack = ancestors(stack)
	if len(stack) == 0 {
		return false
	}
	switch node := stack[len(stack)-1].(type) {
	case *ast.AssignStmt:
		return assigned(node.Lhs, ident)
	case *ast.IncDecStmt:
		return same(node.X, ident)
	case *ast.RangeStmt:
		return same(node.Key, ident) || same(node.Value, ident)
	}
	return false
}

// mutated reports whether the identifier at the top of stack is assigned or
// has its address taken. Replacing either use with the finalizer argument
// would change what the finalizer does.
func mutated(ident *ast.Ident, stack []ast.Node) bool {
	if written(ident, stack) {
		return true
	}
	stack = ancestors(stack)
	if len(stack) == 0 {
		return false
	}
	node, ok := stack[len(stack)-1].(*ast.UnaryExpr)
	return ok && node.Op == token.AND && same(node.X, ident)
}

func ancestors(stack []ast.Node) []ast.Node {
	for len(stack) > 0 {
		if _, paren := stack[len(stack)-1].(*ast.ParenExpr); !paren {
			break
		}
		stack = stack[:len(stack)-1]
	}
	return stack
}

func assigned(exprs []ast.Expr, ident *ast.Ident) bool {
	for _, expr := range exprs {
		if same(expr, ident) {
			return true
		}
	}
	return false
}

func same(expr ast.Expr, ident *ast.Ident) bool {
	if expr == nil {
		return false
	}
	id, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && id == ident
}

// The object can change through the write or the address, so replacing its
// uses would change behavior.
func mutatedObject(pass *analysis.Pass, object types.Object) bool {
	for _, file := range pass.Files {
		var (
			stack []ast.Node
			found bool
		)
		ast.Inspect(file, func(n ast.Node) bool {
			if found {
				return false
			}
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			ident, ok := n.(*ast.Ident)
			if ok {
				used := pass.TypesInfo.Uses[ident]
				if used == object && mutated(ident, stack) {
					found = true
					return false
				}
			}
			stack = append(stack, n)
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// Replacing a written parameter with the finalizer argument changes what the
// finalizer does.
func parameterMutated(pass *analysis.Pass, fn *ast.FuncLit) bool {
	if len(fn.Type.Params.List) != 1 {
		return false
	}
	field := fn.Type.Params.List[0]
	if len(field.Names) != 1 || field.Names[0].Name == "_" {
		return false
	}
	object := pass.TypesInfo.Defs[field.Names[0]]
	return object != nil && mutatedObject(pass, object)
}

// mayHold reports whether t can hold a value of target.
func mayHold(t, target types.Type) bool {
	return holds(t, target, make(map[types.Type]bool))
}

func holds(t, target types.Type, seen map[types.Type]bool) bool {
	t = types.Unalias(t)
	if seen[t] {
		return false
	}
	seen[t] = true
	if types.AssignableTo(target, t) {
		return true
	}
	switch t := t.Underlying().(type) {
	case *types.Interface:
		return true
	case *types.Signature:
		return true
	case *types.Pointer:
		return holds(t.Elem(), target, seen)
	case *types.Array:
		return holds(t.Elem(), target, seen)
	case *types.Slice:
		return holds(t.Elem(), target, seen)
	case *types.Chan:
		return holds(t.Elem(), target, seen)
	case *types.Map:
		return holds(t.Key(), target, seen) || holds(t.Elem(), target, seen)
	case *types.Struct:
		for field := range t.Fields() {
			if holds(field.Type(), target, seen) {
				return true
			}
		}
	}
	return false
}

// The other local may alias the object, so replacing the direct use does
// not break the cycle.
func capturedOther(pass *analysis.Pass, fn *ast.FuncLit, object types.Object) bool {
	var other bool
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if other {
			return false
		}
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		used := pass.TypesInfo.Uses[ident]
		variable, ok := used.(*types.Var)
		if !ok || variable.IsField() || used == object {
			return true
		}
		if used.Pkg() != pass.Pkg || used.Parent() == pass.Pkg.Scope() {
			return true
		}
		if used.Pos() >= fn.Pos() && used.Pos() < fn.End() {
			return true
		}
		if !mayHold(used.Type(), object.Type()) {
			return true
		}
		other = true
		return false
	})
	return other
}

// editSpan is a byte range within one line and the text that replaces it.
type editSpan struct {
	lo, hi int
	text   string
}

// A line that already exceeded the width is not checked again.
func fits(pass *analysis.Pass, edits []analysis.TextEdit) bool {
	if linelen.Config.Len <= 0 || len(edits) == 0 {
		return true
	}
	lines, filename, ok := sourceLines(pass, edits[0].Pos)
	if !ok {
		return false
	}
	byLine, ok := spansByLine(pass, filename, lines, edits)
	if !ok {
		return false
	}
	for line, spans := range byLine {
		if !linelen.CheckLine(lines[line-1]) {
			continue
		}
		text, ok := rewrite(lines[line-1], spans)
		if !ok || !linelen.CheckLine(text) {
			return false
		}
	}
	return true
}

func sourceLines(pass *analysis.Pass, pos token.Pos) ([][]byte, string, bool) {
	var (
		start    = pass.Fset.PositionFor(pos, false)
		src, err = pass.ReadFile(start.Filename)
	)
	if err != nil {
		return nil, "", false
	}
	return bytes.Split(src, []byte("\n")), start.Filename, true
}

func spansByLine(pass *analysis.Pass, filename string, lines [][]byte, edits []analysis.TextEdit) (map[int][]editSpan, bool) {
	byLine := make(map[int][]editSpan)
	for _, edit := range edits {
		from, to := pass.Fset.PositionFor(edit.Pos, false),
			pass.Fset.PositionFor(edit.End, false)
		if from.Filename != filename || from.Line != to.Line {
			return nil, false
		}
		if from.Line < 1 || from.Line > len(lines) {
			return nil, false
		}
		line := lines[from.Line-1]
		lo, hi := from.Column-1, to.Column-1
		if lo < 0 || hi < lo || hi > len(line) {
			return nil, false
		}
		byLine[from.Line] = append(byLine[from.Line], editSpan{
			lo:   lo,
			hi:   hi,
			text: string(edit.NewText),
		})
	}
	return byLine, true
}

func rewrite(line []byte, spans []editSpan) ([]byte, bool) {
	slices.SortFunc(spans, func(x, y editSpan) int { return x.lo - y.lo })
	for i := 1; i < len(spans); i++ {
		if spans[i].lo < spans[i-1].hi {
			return nil, false
		}
	}
	text := line
	for _, span := range slices.Backward(spans) {
		text = append(
			append(append([]byte(nil), text[:span.lo]...), span.text...),
			text[span.hi:]...,
		)
	}
	return text, true
}
