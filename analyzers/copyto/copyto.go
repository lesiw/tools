// Package copyto provides an analysis.Analyzer that reports a loop that
// copies one slice into another element by element.
//
// The "Appending to and copying slices" section of the spec says that copy
// copies the elements of the source into the destination and returns the
// number copied (https://go.dev/ref/spec#Appending_and_copying_slices). A
// loop that writes each element at the index it reads copies one element per
// iteration.
//
// The loop must declare its index and copy every element the source holds:
// either a range over the source or over len(source), or a counted loop from
// zero to len(source). The body is exactly one assignment to an element of
// the destination. The source and destination must be slices with identical
// element types, and neither expression may mention the loop's variables.
// Both must be free of side effects, so a range over a call other than a
// conversion is left alone. Every form evaluates the destination on each
// pass, and the index and counted forms evaluate the source there too. The
// destination in every form, and the source in those two, must keep the same
// slice after a pass writes an element. A scope that declares copy is left
// alone, because the fix would not call the builtin. A loop whose len call
// resolves to another declaration is left alone, and a guarded fix is
// withheld for the same reason.
//
// The fix replaces the loop with copy(dst, src), and is withheld where the
// edit would delete a comment. A loop whose destination is shorter than its
// source panics on the first element past the destination's length, and copy
// writes at most the destination's length. A loop over overlapping slices can
// read an element it has already written, and copy is defined for overlapping
// slices.
// When the destination is not an identifier, the fix checks the source length
// first, so the destination is evaluated only when the loop runs. The README
// catalogs every check, exemption, and fix.
//
// The rule is adapted from staticcheck's S1001, which replaces a copying loop
// with copy or an array assignment. This rule covers slices and reports the
// counted loop, both range forms, and a range over the element count.
package copyto

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
)

const (
	simple = "loop copies one slice into another element by element; use copy"
	caveat = ", which writes at most the destination's length and allows overlap"
)

var Analyzer = &analysis.Analyzer{
	Name: "copyto",
	Doc:  "reports a loop that copies one slice into another element by element",
	Run:  run,
}

// loop is one loop that copies a slice.
type loop struct {
	pos, end token.Pos // the span of the loop
	index    *ast.Ident
	source   ast.Expr
	target   ast.Expr
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			l, ok := loopOf(pass, n)
			if ok && !shadowed(pass, l.pos, "copy") {
				pass.Report(diagnose(pass, file, l))
			}
			return true
		})
	}
	return nil, nil
}

// diagnose returns the diagnostic for one loop and adds the rewrite as its fix
// when one can be applied.
func diagnose(pass *analysis.Pass, file *ast.File, l loop) analysis.Diagnostic {
	var (
		source, sourceOK = astexpr.OneLine(ast.Unparen(l.source))
		target, targetOK = astexpr.OneLine(ast.Unparen(l.target))
	)
	if !sourceOK || !targetOK {
		return analysis.Diagnostic{
			Pos:     l.pos,
			End:     l.end,
			Message: simple + caveat,
		}
	}
	call := "copy(" + target + ", " + source + ")"
	d := analysis.Diagnostic{
		Pos: l.pos,
		End: l.end,
		Message: "loop copies " + source + " into " + target +
			" element by element; use " + call + caveat,
	}
	if commented(file, l) {
		return d
	}
	lead, indented := indent(pass, l.pos)
	text, message := call, "Replace with "+call
	if !named(l.target) {
		unsafe := !repeatable(l.source) || shadowed(pass, l.pos, "len")
		if unsafe || !indented {
			return d
		}
		head := "if len(" + source + ") > 0 {"
		tail := lead + "\t" + call
		text = head + "\n" + tail + "\n" + lead + "}"
		message = "Wrap " + call + " in a length check"
	} else if !indented {
		return d
	}
	if !fits(pass, l, text) {
		return d
	}
	d.SuggestedFixes = []analysis.SuggestedFix{{
		Message: message,
		TextEdits: []analysis.TextEdit{{
			Pos:     l.pos,
			End:     l.end,
			NewText: []byte(text),
		}},
	}}
	return d
}

// fits reports whether the rewritten line or lines are within the width
// linelen measures. The text replaces the loop, so the bytes before and after
// it on its lines are measured too.
func fits(pass *analysis.Pass, l loop, text string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	tokFile := pass.Fset.File(l.pos)
	if tokFile == nil {
		return false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return false
	}
	startLine := pass.Fset.PositionFor(l.pos, false).Line
	endLine := pass.Fset.PositionFor(l.end, false).Line
	start := tokFile.Offset(tokFile.LineStart(startLine))
	end := tokFile.Offset(tokFile.LineStart(endLine))
	at := tokFile.Offset(l.pos)
	to := tokFile.Offset(l.end)
	if endLine < tokFile.LineCount() {
		end = tokFile.Offset(tokFile.LineStart(endLine+1)) - 1
	} else {
		end = len(src)
	}
	if start < 0 || end < 0 || at < start || to > end || end > len(src) {
		return false
	}
	rewritten := string(src[start:at]) + text + string(src[to:end])
	for line := range strings.SplitSeq(rewritten, "\n") {
		if !linelen.CheckLine([]byte(line)) {
			return false
		}
	}
	return true
}

// indent returns the whitespace that leads pos's line, when the line holds
// nothing else before pos.
func indent(pass *analysis.Pass, pos token.Pos) (string, bool) {
	tokFile := pass.Fset.File(pos)
	if tokFile == nil {
		return "", false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return "", false
	}
	line := pass.Fset.PositionFor(pos, false).Line
	start := tokFile.Offset(tokFile.LineStart(line))
	at := tokFile.Offset(pos)
	if start < 0 || at < start || at > len(src) {
		return "", false
	}
	if strings.TrimSpace(string(src[start:at])) != "" {
		return "", false
	}
	return string(src[start:at]), true
}

// named reports whether e is an identifier, which can be evaluated without a
// panic in every context.
func named(e ast.Expr) bool {
	_, ok := ast.Unparen(e).(*ast.Ident)
	return ok
}

// repeatable reports whether e is a simple path that can be evaluated twice
// without allocating. A conversion and a composite literal are rejected even
// when they read the same elements.
func repeatable(e ast.Expr) bool {
	ok := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.CallExpr, *ast.CompositeLit, *ast.FuncLit:
			ok = false
		}
		return ok
	})
	return ok
}

// commented reports whether a comment falls inside l, where the edit would
// delete it along with the loop.
func commented(file *ast.File, l loop) bool {
	for _, group := range file.Comments {
		if group.Pos() < l.end && group.End() > l.pos {
			return true
		}
	}
	return false
}

// shadowed reports whether name resolves to something other than the builtin
// at pos.
func shadowed(pass *analysis.Pass, pos token.Pos, name string) bool {
	_, obj := pass.Pkg.Scope().Innermost(pos).LookupParent(name, pos)
	return obj != types.Universe.Lookup(name)
}

// loopOf returns n as a loop that copies a slice, and reports false for
// anything else.
func loopOf(pass *analysis.Pass, n ast.Node) (loop, bool) {
	var l loop
	switch n := n.(type) {
	case *ast.RangeStmt:
		l = loop{pos: n.Pos(), end: n.End()}
		l.index, l.source, l.target = ranged(pass, n)
	case *ast.ForStmt:
		l = loop{pos: n.Pos(), end: n.End()}
		l.index, l.source, l.target = counted(pass, n)
	default:
		return loop{}, false
	}
	if l.source == nil || !copied(pass, l) {
		return loop{}, false
	}
	return l, true
}

// ranged returns the index, source, and destination of a range loop that
// copies its elements, and nils for any other statement.
func ranged(pass *analysis.Pass, stmt *ast.RangeStmt) (_ *ast.Ident, _, _ ast.Expr) {
	index, value, ok := rangeVars(stmt)
	if !ok {
		return nil, nil, nil
	}
	target, rhs, ok := indexed(pass, stmt.Body, index)
	if !ok || !targetStable(pass, target) {
		return nil, nil, nil
	}
	source := stmt.X
	if value == nil {
		if operand, ok := lenOperand(pass, source); ok {
			source = operand
		}
		var (
			stableSource = stable(pass, source) && aliasFree(pass, source)
			rebound      = rebinds(pass, source, target)
		)
		if !stableSource || rebound || !reads(pass, rhs, source, index) {
			return nil, nil, nil
		}
	} else if !astexpr.Same(pass, rhs, value) {
		return nil, nil, nil
	}
	if !invariant(pass, source, target.X, index, value) {
		return nil, nil, nil
	}
	return index, source, target.X
}

// rangeVars returns the index and value a range loop declares, with a blank
// value read as absent.
func rangeVars(stmt *ast.RangeStmt) (_, _ *ast.Ident, _ bool) {
	if stmt.Tok != token.DEFINE {
		return nil, nil, false
	}
	index, ok := ast.Unparen(stmt.Key).(*ast.Ident)
	if !ok || index.Name == "_" {
		return nil, nil, false
	}
	if stmt.Value == nil {
		return index, nil, true
	}
	value, ok := ast.Unparen(stmt.Value).(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	if value.Name == "_" {
		return index, nil, true
	}
	return index, value, true
}

// counted returns the index, source, and destination of a counted loop that
// copies a slice, and nils for any other statement.
func counted(pass *analysis.Pass, stmt *ast.ForStmt) (_ *ast.Ident, _, _ ast.Expr) {
	index, ok := atZero(pass, stmt.Init)
	if !ok {
		return nil, nil, nil
	}
	source, ok := countedSource(pass, stmt.Cond, index)
	if !ok || !stable(pass, source) || !aliasFree(pass, source) {
		return nil, nil, nil
	}
	post, ok := stmt.Post.(*ast.IncDecStmt)
	if !ok || post.Tok != token.INC || !astexpr.Same(pass, post.X, index) {
		return nil, nil, nil
	}
	target, rhs, ok := indexed(pass, stmt.Body, index)
	stableTarget := ok && targetStable(pass, target)
	rebound := ok && rebinds(pass, source, target)
	if !stableTarget || rebound || !reads(pass, rhs, source, index) {
		return nil, nil, nil
	}
	if !invariant(pass, source, target.X, index, nil) {
		return nil, nil, nil
	}
	return index, source, target.X
}

// countedSource returns the source a counted loop compares its index to,
// when the condition is index < len(source).
func countedSource(pass *analysis.Pass, cond ast.Expr, index *ast.Ident) (ast.Expr, bool) {
	bin, ok := ast.Unparen(cond).(*ast.BinaryExpr)
	if !ok || bin.Op != token.LSS || !astexpr.Same(pass, bin.X, index) {
		return nil, false
	}
	return lenOperand(pass, bin.Y)
}

// indexed returns the destination and the assigned value of a body that is
// one assignment to the element at index.
func indexed(pass *analysis.Pass, body *ast.BlockStmt, index *ast.Ident) (*ast.IndexExpr, ast.Expr, bool) {
	assign, ok := assignment(body)
	if !ok {
		return nil, nil, false
	}
	target, ok := ast.Unparen(assign.Lhs[0]).(*ast.IndexExpr)
	if !ok || !astexpr.Same(pass, target.Index, index) {
		return nil, nil, false
	}
	return target, assign.Rhs[0], true
}

// reads reports whether rhs reads the element of source at index.
func reads(pass *analysis.Pass, rhs, source ast.Expr, index *ast.Ident) bool {
	read, ok := ast.Unparen(rhs).(*ast.IndexExpr)
	if !ok || !astexpr.Same(pass, read.Index, index) {
		return false
	}
	return same(pass, read.X, source)
}

// same reports whether a and b read the same value. astexpr.Same covers the
// simple forms; the printed form covers a call or a slice expression, and the
// identifiers must name the same objects so a name bound by the loop is not
// read as an outer one.
func same(pass *analysis.Pass, a, b ast.Expr) bool {
	if astexpr.Same(pass, a, b) {
		return true
	}
	a, b = ast.Unparen(a), ast.Unparen(b)
	if astexpr.Text(a) != astexpr.Text(b) {
		return false
	}
	var left, right []*ast.Ident
	ast.Inspect(a, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			left = append(left, ident)
		}
		return true
	})
	ast.Inspect(b, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			right = append(right, ident)
		}
		return true
	})
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !sameIdent(pass, left[i], right[i]) {
			return false
		}
	}
	return true
}

// sameIdent reports whether a and b name the same object.
func sameIdent(pass *analysis.Pass, a, b *ast.Ident) bool {
	return pass.TypesInfo.ObjectOf(a) == pass.TypesInfo.ObjectOf(b)
}

// stable reports whether evaluating e twice reads a slice with the same
// elements, even after the loop has written elements of the destination. The
// slice expression may read one element at its root, as rows[k] does, but no
// element below that point, since a pass can replace it. A call is stable when
// it is a conversion or a builtin that reads only its arguments.
func stable(pass *analysis.Pass, e ast.Expr) bool {
	return steady(pass, e, true)
}

// steady is stable with root set on the expression the caller names. Only a
// root index expression may read an element, and only to reach the slice it
// returns.
func steady(pass *analysis.Pass, e ast.Expr, root bool) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident, *ast.BasicLit:
		return true
	case *ast.SelectorExpr:
		return steady(pass, e.X, false)
	case *ast.IndexExpr:
		return root && steady(pass, e.X, false) && bound(pass, e.Index)
	case *ast.SliceExpr:
		return root && steady(pass, e.X, false) && bound(pass, e.Low) &&
			bound(pass, e.High) && bound(pass, e.Max)
	case *ast.StarExpr:
		return steady(pass, e.X, false)
	case *ast.TypeAssertExpr:
		return steady(pass, e.X, false)
	case *ast.CallExpr:
		if astexpr.Effectful(pass, e) {
			return false
		}
		for _, arg := range e.Args {
			if !steady(pass, arg, false) {
				return false
			}
		}
		return true
	}
	return false
}

// rebinds reports whether source and target read from the same root slice and
// a target element can hold the source slice. A pass can then replace the
// source or destination header.
func rebinds(pass *analysis.Pass, source ast.Expr, target *ast.IndexExpr) bool {
	if _, ok := ast.Unparen(source).(*ast.IndexExpr); !ok {
		return false
	}
	var (
		sourceType = pass.TypesInfo.TypeOf(source)
		elemType   = pass.TypesInfo.TypeOf(target)
	)
	return sourceType != nil && elemType != nil &&
		types.AssignableTo(sourceType, elemType)
}

// targetStable reports whether evaluating target twice names the same slice,
// even after the loop writes its elements.
func targetStable(pass *analysis.Pass, target *ast.IndexExpr) bool {
	return stable(pass, target.X) && boundsStable(pass, target.X) &&
		!pointerAlias(pass, target.X, pass.TypesInfo.TypeOf(target)) &&
		!rebinds(pass, target.X, target)
}

// pointerAlias reports whether e reads through a pointer whose pointee has the
// element type, so a write to an element can replace what the pointer names.
func pointerAlias(pass *analysis.Pass, e ast.Expr, elem types.Type) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.StarExpr:
			if ptr, ok := pointerType(pass.TypesInfo.TypeOf(n.X)); ok {
				found = found || overlaps(elem, ptr.Elem())
			}
		case *ast.SelectorExpr:
			if sel, ok := pass.TypesInfo.Selections[n]; ok {
				var (
					ptr, pointer = pointerType(sel.Recv())
					promoted     = sel.Indirect() && len(sel.Index()) > 1
				)
				found = found || promoted ||
					pointer && overlaps(elem, ptr.Elem())
			}
		}
		return !found
	})
	return found
}

// overlaps reports whether a value of t can hold a value of u through a value
// field or an array.
func overlaps(t, u types.Type) bool {
	var (
		same       = types.Identical(t, u)
		underlying = types.Identical(t.Underlying(), u.Underlying())
	)
	if same || underlying {
		return true
	}
	switch t := t.Underlying().(type) {
	case *types.Struct:
		for field := range t.Fields() {
			if overlaps(field.Type(), u) {
				return true
			}
		}
	case *types.Array:
		return overlaps(t.Elem(), u)
	}
	return false
}

// boundsStable reports whether the index and slice bounds in e read no value
// through a pointer.
func boundsStable(pass *analysis.Pass, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.IndexExpr:
		return boundsStable(pass, e.X) && !pointerRead(pass, e.Index)
	case *ast.SliceExpr:
		return boundsStable(pass, e.X) && !pointerRead(pass, e.Low) &&
			!pointerRead(pass, e.High) && !pointerRead(pass, e.Max)
	case *ast.CallExpr:
		if len(e.Args) == 1 {
			return boundsStable(pass, e.Args[0])
		}
	}
	return true
}

// pointerRead reports whether e reads through a pointer.
func pointerRead(pass *analysis.Pass, e ast.Expr) bool {
	if e == nil {
		return false
	}
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.StarExpr:
			found = true
		case *ast.SelectorExpr:
			if sel, ok := pass.TypesInfo.Selections[n]; ok {
				_, pointer := pointerType(sel.Recv())
				found = found || pointer || sel.Indirect()
			}
		}
		return !found
	})
	return found
}

// aliasFree reports whether e reads through a pointer. A pointer can point
// into the destination, so the loop's writes can change what it reads.
func aliasFree(pass *analysis.Pass, e ast.Expr) bool {
	ok := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.StarExpr:
			ok = false
		case *ast.SelectorExpr:
			if sel, found := pass.TypesInfo.Selections[n]; found {
				ok = ok && !sel.Indirect() && !nilable(sel.Recv())
			}
		}
		return ok
	})
	return ok
}

// pointerType returns t as a pointer, including a defined pointer type.
func pointerType(t types.Type) (*types.Pointer, bool) {
	if t == nil {
		return nil, false
	}
	ptr, ok := t.Underlying().(*types.Pointer)
	return ptr, ok
}

// nilable reports whether t is a pointer or an interface.
func nilable(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Interface:
		return true
	}
	return false
}

// bound reports whether e reads no element of a slice, so a write to the
// destination cannot change it.
func bound(pass *analysis.Pass, e ast.Expr) bool {
	if e == nil {
		return true
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident, *ast.BasicLit:
		return true
	case *ast.SelectorExpr:
		return bound(pass, e.X)
	case *ast.UnaryExpr:
		return e.Op != token.ARROW && bound(pass, e.X)
	case *ast.BinaryExpr:
		return bound(pass, e.X) && bound(pass, e.Y)
	case *ast.CallExpr:
		if astexpr.Effectful(pass, e) {
			return false
		}
		for _, arg := range e.Args {
			if !bound(pass, arg) {
				return false
			}
		}
		return true
	}
	return false
}

// invariant reports whether source and target mention neither the index nor
// the value, which the replacement cannot name.
func invariant(pass *analysis.Pass, source, target ast.Expr, index, value *ast.Ident) bool {
	objects := []types.Object{pass.TypesInfo.ObjectOf(index)}
	if value != nil {
		objects = append(objects, pass.TypesInfo.ObjectOf(value))
	}
	for _, obj := range objects {
		if mentions(pass, source, obj) || mentions(pass, target, obj) {
			return false
		}
	}
	return true
}

// atZero returns the index a counted loop declares at zero, and reports false
// for any other init statement.
func atZero(pass *analysis.Pass, stmt ast.Stmt) (*ast.Ident, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	index, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, false
	}
	value := pass.TypesInfo.Types[ast.Unparen(assign.Rhs[0])].Value
	if value == nil || value.Kind() != constant.Int {
		return nil, false
	}
	if constant.Sign(value) != 0 {
		return nil, false
	}
	return index, true
}

// lenOperand returns the operand of x when x is len(operand).
func lenOperand(pass *analysis.Pass, x ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(x).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[ident] != types.Universe.Lookup("len") {
		return nil, false
	}
	return call.Args[0], true
}

// assignment returns the sole statement of body when it is a plain
// assignment with one target and one value.
func assignment(body *ast.BlockStmt) (*ast.AssignStmt, bool) {
	if body == nil || len(body.List) != 1 {
		return nil, false
	}
	assign, ok := body.List[0].(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	return assign, true
}

// copied reports whether the loop's source and destination are slices with
// identical element types and neither has a side effect.
func copied(pass *analysis.Pass, l loop) bool {
	if astexpr.Effectful(pass, l.source) || astexpr.Effectful(pass, l.target) {
		return false
	}
	if !invariant(pass, l.source, l.target, l.index, nil) {
		return false
	}
	source, ok := sliceElem(pass.TypesInfo.TypeOf(l.source))
	if !ok {
		return false
	}
	target, ok := sliceElem(pass.TypesInfo.TypeOf(l.target))
	if !ok {
		return false
	}
	return types.Identical(source, target)
}

// sliceElem returns the element type of a slice, and reports false for any
// other type.
func sliceElem(t types.Type) (types.Type, bool) {
	if t == nil {
		return nil, false
	}
	slice, ok := t.Underlying().(*types.Slice)
	if !ok {
		return nil, false
	}
	return slice.Elem(), true
}

// mentions reports whether e reads obj.
func mentions(pass *analysis.Pass, e ast.Expr, obj types.Object) bool {
	if obj == nil {
		return false
	}
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if pass.TypesInfo.ObjectOf(ident) == obj {
				found = true
			}
		}
		return !found
	})
	return found
}
