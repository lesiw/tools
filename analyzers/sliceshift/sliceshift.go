// Package sliceshift provides an analysis.Analyzer that reports a loop that
// shifts elements within one slice by an offset.
//
// The "Appending to and copying slices" section of the spec says that copy
// copies the elements of the source into the destination and that "[t]he
// result is independent of whether the memory referenced by the arguments
// overlaps" (https://go.dev/ref/spec#Appending_and_copying_slices). A loop
// that reads and writes one slice at neighboring indexes does the same shift,
// and copy does it in one call.
//
// The loop's init statement declares its index, its condition tests the index
// against a bound with a strict comparison, and its post statement moves the
// index by one. The index may be on either side of the comparison. The body is
// one assignment between two elements of the same slice. The destination index
// is the loop index, and the source index is that index plus an offset, in
// either order, or minus an offset for the reverse direction. The offset is an
// identifier or a literal.
//
// A forward loop reads the element ahead of the one it writes, and a reverse
// loop reads the element behind it. In both directions a read reaches an
// element the loop has not written yet, so copy gives the same result. A
// forward loop with a non-constant offset is reported only when its index
// starts at zero; a negative offset makes the first read negative, and the
// assignment evaluates its indexes before it writes
// (https://go.dev/ref/spec#Assignment_statements), so the loop panics first.
// The bounds and the offset must not read an element of the slice, since a
// write could change them while the loop runs. A scope that declares copy is
// left alone, because the replacement would not call the builtin.
//
// The fix replaces the loop with copy when every index the loop visits is
// within the slice's length. That holds for a forward loop that starts at a
// non-negative constant and stops at len(slice)-k with k a constant at least
// the offset, and for a reverse loop that starts at len(slice)-1-c with c a
// constant over a stop of at least the offset minus one, and the offset is a
// positive constant. The loop's condition becomes a guard, so the copy is
// skipped when the loop would not run.
//
// Any other bound is reported without a fix. Copy over slices whose bounds are
// not known to be within the slice's length would change where the loop panics
// (https://go.dev/ref/spec#Index_expressions) and which elements it wrote
// first. The fix is also withheld when start+offset, stop+1, or a derived
// bound would exceed the target's largest int, since a constant expression
// that overflows does not compile
// (https://go.dev/ref/spec#Constant_expressions). The fix is withheld where
// the edit would delete a comment, where the replacement would run past the
// width, where the loop does not begin its line, and where an expression has
// no one-line form. The diagnostic is still reported.
//
// The rule is adapted from staticcheck's S1018, which reports the forward
// loop that starts at zero. This rule also reports the reverse loop and the
// wider operand and bound forms.
package sliceshift

import (
	"go/ast"
	"go/constant"
	"go/format"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "sliceshift",
	Doc:  "reports a loop that shifts elements within one slice",
	Run:  run,
}

// shift is one loop that slides a slice's elements.
type shift struct {
	loop     *ast.ForStmt
	slice    *ast.Ident
	start    ast.Expr // the index's first value
	stop     ast.Expr // the value the condition stops before
	offset   ast.Expr // the distance between the read and the write
	backward bool
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			loop, ok := n.(*ast.ForStmt)
			if !ok {
				return true
			}
			s, ok := shiftOf(pass, loop)
			if !ok || shadowed(pass, loop.Pos(), "copy") {
				return true
			}
			pass.Report(diagnose(pass, file, s))
			return true
		})
	}
	return nil, nil
}

// shiftOf returns loop as a shift over a slice, and reports false for
// anything else.
func shiftOf(pass *analysis.Pass, loop *ast.ForStmt) (shift, bool) {
	index, start, stop, backward, ok := header(pass, loop)
	if !ok || !quiet(pass, start, stop, index) {
		return shift{}, false
	}
	start, stop = ast.Unparen(start), ast.Unparen(stop)
	dest, source, ok := body(pass, loop, index)
	if !ok || !indexFree(pass, start, dest.X) {
		return shift{}, false
	}
	if !indexFree(pass, stop, dest.X) {
		return shift{}, false
	}
	offset, ok := shiftOffset(pass, backward, source.Index, index, start)
	if !ok {
		return shift{}, false
	}
	if !iterates(pass, start, stop, backward) {
		return shift{}, false
	}
	return shift{
		loop:     loop,
		slice:    ast.Unparen(dest.X).(*ast.Ident),
		start:    start,
		stop:     stop,
		offset:   offset,
		backward: backward,
	}, true
}

// quiet reports whether the bounds mention neither the index nor a call that
// could read a different value each time.
func quiet(pass *analysis.Pass, start, stop ast.Expr, index *ast.Ident) bool {
	if astexpr.Mentions(pass, start, index) {
		return false
	}
	if astexpr.Mentions(pass, stop, index) {
		return false
	}
	return !astexpr.Effectful(pass, start) && !astexpr.Effectful(pass, stop)
}

// shiftOffset returns the source's offset from the index, when the loop's
// direction and the offset's value make the read reach an element the loop
// has not written.
func shiftOffset(pass *analysis.Pass, backward bool, e ast.Expr, index *ast.Ident, start ast.Expr) (ast.Expr, bool) {
	sign, offset, ok := indexOffset(pass, e, index)
	if !ok || offset == nil {
		return nil, false
	}
	offset = ast.Unparen(offset)
	if !simple(offset) {
		return nil, false
	}
	value, known := constantValue(pass, offset)
	if backward {
		// A reverse loop reads the element below the one it writes,
		// so the offset is subtracted and must be a positive constant.
		if sign != -1 || !known || value <= 0 {
			return nil, false
		}
		return offset, true
	}
	if sign != 1 || (known && value <= 0) {
		return nil, false
	}
	// Without a constant, the offset's sign is only known at run time. At a
	// zero start a negative offset makes the first read negative, so a loop
	// that writes anything uses a non-negative offset. At another start a
	// negative offset could read an element the loop already wrote.
	if !known && !zero(pass, start) {
		return nil, false
	}
	return offset, true
}

// header returns the loop's index, its first value, the value its condition
// stops at, and whether the index decreases. It reports false for a loop that
// does not move the index by one.
func header(pass *analysis.Pass, loop *ast.ForStmt) (index *ast.Ident, start, stop ast.Expr, backward, ok bool) {
	index, start, ok = declared(loop.Init)
	if !ok {
		return nil, nil, nil, false, false
	}
	backward, stop, ok = direction(pass, loop.Cond, loop.Post, index)
	if !ok {
		return nil, nil, nil, false, false
	}
	return index, start, stop, backward, true
}

// declared returns the index and first value of a loop's init statement.
func declared(init ast.Stmt) (index *ast.Ident, start ast.Expr, ok bool) {
	assign, ok := init.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return nil, nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, nil, false
	}
	index, ok = ast.Unparen(assign.Lhs[0]).(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	return index, assign.Rhs[0], true
}

// direction returns whether the index decreases and the value the condition
// stops at.
func direction(pass *analysis.Pass, cond ast.Expr, post ast.Stmt, index *ast.Ident) (backward bool, stop ast.Expr, ok bool) {
	bin, ok := ast.Unparen(cond).(*ast.BinaryExpr)
	if !ok {
		return false, nil, false
	}
	step, ok := post.(*ast.IncDecStmt)
	if !ok || !astexpr.Same(pass, step.X, index) {
		return false, nil, false
	}
	switch {
	case bin.Op == token.LSS && step.Tok == token.INC &&
		astexpr.Same(pass, bin.X, index):
		return false, bin.Y, true
	case bin.Op == token.GTR && step.Tok == token.INC &&
		astexpr.Same(pass, bin.Y, index):
		return false, bin.X, true
	case bin.Op == token.GTR && step.Tok == token.DEC &&
		astexpr.Same(pass, bin.X, index):
		return true, bin.Y, true
	case bin.Op == token.LSS && step.Tok == token.DEC &&
		astexpr.Same(pass, bin.Y, index):
		return true, bin.X, true
	}
	return false, nil, false
}

// body returns the destination and source of the loop's one assignment, when
// the destination indexes the same slice with the loop index.
func body(pass *analysis.Pass, loop *ast.ForStmt, index *ast.Ident) (dest, source *ast.IndexExpr, ok bool) {
	if loop.Body == nil || len(loop.Body.List) != 1 {
		return nil, nil, false
	}
	assign, ok := loop.Body.List[0].(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return nil, nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, nil, false
	}
	dest, ok = ast.Unparen(assign.Lhs[0]).(*ast.IndexExpr)
	if !ok {
		return nil, nil, false
	}
	source, ok = ast.Unparen(assign.Rhs[0]).(*ast.IndexExpr)
	if !ok {
		return nil, nil, false
	}
	ident, ok := ast.Unparen(dest.X).(*ast.Ident)
	if !ok || !slice(pass, ident) || !astexpr.Same(pass, dest.X, source.X) {
		return nil, nil, false
	}
	sign, _, ok := indexOffset(pass, dest.Index, index)
	if !ok || sign != 0 {
		return nil, nil, false
	}
	return dest, source, true
}

// indexOffset returns the sign and the offset of index relative to the loop's
// index: zero for the index itself, positive for index+offset or offset+index,
// and negative for index-offset.
func indexOffset(pass *analysis.Pass, e ast.Expr, index *ast.Ident) (sign int, offset ast.Expr, ok bool) {
	e = ast.Unparen(e)
	if astexpr.Same(pass, e, index) {
		return 0, nil, true
	}
	bin, ok := e.(*ast.BinaryExpr)
	if !ok {
		return 0, nil, false
	}
	switch bin.Op {
	case token.ADD:
		if relative(pass, bin.X, bin.Y, index) {
			return 1, bin.Y, true
		}
		if relative(pass, bin.Y, bin.X, index) {
			return 1, bin.X, true
		}
	case token.SUB:
		if relative(pass, bin.X, bin.Y, index) {
			return -1, bin.Y, true
		}
	}
	return 0, nil, false
}

// relative reports whether e is the loop's index and other does not mention
// it.
func relative(pass *analysis.Pass, e, other ast.Expr, index *ast.Ident) bool {
	return astexpr.Same(pass, e, index) &&
		!astexpr.Mentions(pass, other, index)
}

// diagnose returns the diagnostic for one shift and adds the rewrite as its
// fix when one can be applied.
func diagnose(pass *analysis.Pass, file *ast.File, s shift) analysis.Diagnostic {
	var (
		message = "loop shifts elements within one slice"
		diag    = analysis.Diagnostic{
			Pos:     s.loop.Pos(),
			End:     s.loop.End(),
			Message: message,
		}
		fixable = bounded(pass, s)
	)
	if !fixable || commented(file, s.loop) {
		return diag
	}
	call, ok := copyCall(pass, s)
	if !ok {
		return diag
	}
	text, ok := rewrite(pass, s, call)
	if !ok || !fits(pass, s.loop, text) {
		return diag
	}
	diag.Message = "loop shifts elements within one slice; use " + call
	diag.SuggestedFixes = []analysis.SuggestedFix{{
		Message: "Replace the loop with copy",
		TextEdits: []analysis.TextEdit{{
			Pos:     s.loop.Pos(),
			End:     s.loop.End(),
			NewText: []byte(text),
		}},
	}}
	return diag
}

// rewrite returns the text that replaces the loop. The condition becomes a
// guard, since a slice expression with a low bound past its high one panics
// where the loop would not run.
func rewrite(pass *analysis.Pass, s shift, call string) (string, bool) {
	lead, ok := indent(pass, s.loop.Pos())
	if !ok {
		return "", false
	}
	cond, ok := condition(pass, s)
	if !ok {
		return "", false
	}
	var b strings.Builder
	b.WriteString("if ")
	b.WriteString(cond)
	b.WriteString(" {\n")
	b.WriteString(lead)
	b.WriteByte('\t')
	b.WriteString(call)
	b.WriteByte('\n')
	b.WriteString(lead)
	b.WriteByte('}')
	return b.String(), true
}

// condition returns the loop's condition over its first and last values.
func condition(_ *analysis.Pass, s shift) (string, bool) {
	op := token.LSS
	if s.backward {
		op = token.GTR
	}
	text := astexpr.Text(&ast.BinaryExpr{X: s.start, Op: op, Y: s.stop})
	if strings.ContainsRune(text, '\n') {
		return "", false
	}
	return text, true
}

// bounded reports whether every index the loop visits is within the slice's
// length, so copy cannot change where the loop panics or how much it writes
// first. A forward loop starts at a non-negative constant and stops at
// len(slice) - k with k at least the offset. A reverse loop starts at
// len(slice) - 1 - c over a stop of at least the offset minus one.
func bounded(pass *analysis.Pass, s shift) bool {
	offset, ok := constantValue(pass, s.offset)
	if !ok || offset <= 0 {
		return false
	}
	limit := intLimit(pass)
	if s.backward {
		k, ok := lenMinus(pass, s.start, s.slice, 1)
		if !ok || k < 0 || k > limit {
			return false
		}
		stop, ok := constantValue(pass, s.stop)
		return ok && stop >= offset-1 && stop < limit
	}
	start, ok := constantValue(pass, s.start)
	if !ok || start < 0 || start > limit-offset {
		return false
	}
	k, ok := lenMinus(pass, s.stop, s.slice, 0)
	return ok && k >= offset && k <= limit
}

// intLimit returns the largest int on the target.
func intLimit(pass *analysis.Pass) int64 {
	if pass.TypesSizes == nil {
		return 0
	}
	size := pass.TypesSizes.Sizeof(types.Typ[types.Int])
	if size >= 8 {
		return maxInt64
	}
	return 1<<(8*size-1) - 1
}

// lenMinus returns k when e is len(slice) - sub - k.
func lenMinus(pass *analysis.Pass, e, slice ast.Expr, sub int64) (int64, bool) {
	e = ast.Unparen(e)
	if isLen(pass, e, slice) {
		return -sub, true
	}
	bin, ok := e.(*ast.BinaryExpr)
	if !ok || bin.Op != token.SUB {
		return 0, false
	}
	k, ok := constantValue(pass, bin.Y)
	if !ok {
		return 0, false
	}
	k1, ok := lenMinus(pass, bin.X, slice, sub)
	if !ok {
		return 0, false
	}
	return checkedAdd(k1, k)
}

// checkedAdd returns a + b when it fits an int64.
func checkedAdd(a, b int64) (int64, bool) {
	const (
		high = int64(^uint64(0) >> 1)
		low  = -high - 1
	)
	if b > 0 && a > high-b {
		return 0, false
	}
	if b < 0 && a < low-b {
		return 0, false
	}
	return a + b, true
}

// isLen reports whether e is len(slice).
func isLen(pass *analysis.Pass, e, slice ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[ident] != types.Universe.Lookup("len") {
		return false
	}
	return astexpr.Same(pass, call.Args[0], slice)
}

// copyCall returns the copy call that replaces the loop, and reports false
// when an expression has no one-line form.
func copyCall(pass *analysis.Pass, s shift) (string, bool) {
	offset, ok := constantValue(pass, s.offset)
	if !ok {
		return "", false
	}
	var destLow, destHigh, sourceLow, sourceHigh ast.Expr
	if s.backward {
		c, ok := lenMinus(pass, s.start, s.slice, 1)
		if !ok {
			return "", false
		}
		stop, ok := constantValue(pass, s.stop)
		if !ok {
			return "", false
		}
		// The loop runs from start down to stop+1. The source index
		// is offset less than the destination index.
		destLow = intLit(stop + 1)
		destHigh = subtract(lenCall(s.slice), c)
		sourceLow = intLit(stop + 1 - offset)
		sourceHigh = subtract(subtract(lenCall(s.slice), c), offset)
	} else {
		k, ok := lenMinus(pass, s.stop, s.slice, 0)
		if !ok {
			return "", false
		}
		destHigh = s.stop
		start, _ := constantValue(pass, s.start)
		if start == 0 {
			sourceLow = s.offset
		} else {
			destLow = s.start
			sourceLow = intLit(start + offset)
		}
		sourceHigh = subtract(lenCall(s.slice), k-offset)
	}
	dest := sliceText(pass, s.slice, destLow, destHigh)
	source := sliceText(pass, s.slice, sourceLow, sourceHigh)
	if dest == "" || source == "" || strings.ContainsAny(dest+source, "\n") {
		return "", false
	}
	return formatCall("copy(" + dest + ", " + source + ")")
}

// formatCall returns text as gofmt writes it, and reports false when it holds
// a newline.
func formatCall(text string) (string, bool) {
	const prefix = "package p\n\nvar _ = "
	out, err := format.Source([]byte(prefix + text + "\n"))
	if err != nil {
		return "", false
	}
	rest, ok := strings.CutPrefix(string(out), prefix)
	if !ok {
		return "", false
	}
	rest = strings.TrimSuffix(rest, "\n")
	if strings.ContainsRune(rest, '\n') {
		return "", false
	}
	return rest, true
}

// maxInt64 is the largest int64, which intLimit returns when int is 64 bits.
const maxInt64 = int64(^uint64(0) >> 1)

// lenCall returns len(slice).
func lenCall(slice ast.Expr) ast.Expr {
	return &ast.CallExpr{Fun: ast.NewIdent("len"), Args: []ast.Expr{slice}}
}

// sliceText prints the slice expression over e with the given bounds. A bound
// that is a constant zero is left out, as in xs[:n].
func sliceText(pass *analysis.Pass, e, low, high ast.Expr) string {
	if zero(pass, low) {
		low = nil
	}
	return astexpr.Text(&ast.SliceExpr{X: e, Low: low, High: high})
}

// subtract returns e - n, or e when n is zero.
func subtract(e ast.Expr, n int64) ast.Expr {
	if n == 0 {
		return e
	}
	return &ast.BinaryExpr{X: e, Op: token.SUB, Y: intLit(n)}
}

// intLit returns the integer literal for n. The callers pass non-negative
// values; a negative value becomes a unary minus so the node stays valid.
func intLit(n int64) ast.Expr {
	if n < 0 {
		magnitude := uint64(-(n + 1)) + 1
		return &ast.UnaryExpr{Op: token.SUB, X: uintLiteral(magnitude)}
	}
	return uintLiteral(uint64(n))
}

// uintLiteral returns an integer literal for n.
func uintLiteral(n uint64) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(n, 10)}
}

// constantValue returns the value of e when it is an integer constant.
func constantValue(pass *analysis.Pass, e ast.Expr) (int64, bool) {
	tv := pass.TypesInfo.Types[ast.Unparen(e)]
	if tv.Value == nil || tv.Value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(tv.Value)
}

// zero reports whether e is the constant zero.
func zero(pass *analysis.Pass, e ast.Expr) bool {
	if e == nil {
		return true
	}
	if lit, ok := ast.Unparen(e).(*ast.BasicLit); ok {
		return lit.Kind == token.INT && lit.Value == "0"
	}
	value, ok := constantValue(pass, e)
	return ok && value == 0
}

// iterates reports whether the loop can run at least once, when both bounds
// are constants.
func iterates(pass *analysis.Pass, start, stop ast.Expr, backward bool) bool {
	var (
		first, ok1 = constantValue(pass, start)
		last, ok2  = constantValue(pass, stop)
	)
	if !ok1 || !ok2 {
		return true
	}
	if backward {
		return first > last
	}
	return first < last
}

// simple reports whether e is an identifier or a literal, which cannot change
// when the loop writes an element.
func simple(e ast.Expr) bool {
	switch ast.Unparen(e).(type) {
	case *ast.Ident, *ast.BasicLit:
		return true
	}
	return false
}

// indexFree reports whether e reads no element of the slice.
func indexFree(pass *analysis.Pass, e, slice ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		index, ok := n.(*ast.IndexExpr)
		if !ok {
			return true
		}
		if astexpr.Mentions(pass, index.X, slice) {
			found = true
		}
		return !found
	})
	return !found
}

// slice reports whether ident has a slice type.
func slice(pass *analysis.Pass, ident *ast.Ident) bool {
	t := pass.TypesInfo.TypeOf(ident)
	if t == nil {
		return false
	}
	_, ok := types.Unalias(t).Underlying().(*types.Slice)
	return ok
}

// shadowed reports whether name resolves to something other than the builtin
// at pos.
func shadowed(pass *analysis.Pass, pos token.Pos, name string) bool {
	_, obj := pass.Pkg.Scope().Innermost(pos).LookupParent(name, pos)
	return obj != types.Universe.Lookup(name)
}

// commented reports whether a comment sits inside the loop, where the edit
// would delete it along with the loop.
func commented(file *ast.File, loop *ast.ForStmt) bool {
	for _, group := range file.Comments {
		if group.Pos() < loop.End() && group.End() > loop.Pos() {
			return true
		}
	}
	return false
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

// fits reports whether the rewritten line or lines are within the width
// linelen measures. The text replaces the loop, so the bytes before and after
// it on its lines are measured too.
func fits(pass *analysis.Pass, loop *ast.ForStmt, text string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	tokFile := pass.Fset.File(loop.Pos())
	if tokFile == nil {
		return false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return false
	}
	startLine := pass.Fset.PositionFor(loop.Pos(), false).Line
	endLine := pass.Fset.PositionFor(loop.End(), false).Line
	start := tokFile.Offset(tokFile.LineStart(startLine))
	at := tokFile.Offset(loop.Pos())
	to := tokFile.Offset(loop.End())
	end := len(src)
	if endLine < tokFile.LineCount() {
		end = tokFile.Offset(tokFile.LineStart(endLine+1)) - 1
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
