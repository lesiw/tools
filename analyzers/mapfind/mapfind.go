// Package mapfind provides an analysis.Analyzer that reports a loop that
// searches a map for one key.
//
// The specification's "Index expressions" section says an index expression on
// a map yields the element for a key, and that its two-result form adds an
// untyped boolean that is true when the key is present. "Go maps in action"
// says a Go map implements a hash table, so a lookup reaches the entry
// directly, where a range visits every key.
//
// The rewrite replaces the loop with an if whose init indexes the map and
// whose block holds the return. The report and the fix are withheld wherever
// that replacement is not the same search or not the same program; the README
// lists the cases.
//
// The rule is adapted from revive's inefficient-map-lookup, which reports the
// comparison without reading the if body and offers no fix.
package mapfind

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"slices"
	"strconv"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

var Analyzer = &analysis.Analyzer{
	Name: "mapfind",
	Doc:  "reports a loop that searches a map for one key",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if rng, ok := n.(*ast.RangeStmt); ok {
				examine(pass, file, rng)
			}
			return true
		})
	}
	return nil, nil
}

// examine reports rng when it searches a map for one key.
func examine(pass *analysis.Pass, file *ast.File, rng *ast.RangeStmt) {
	f := finder{pass: pass, file: file, rng: rng}
	if !f.search() {
		return
	}
	diagnostic := analysis.Diagnostic{
		Pos:     rng.Pos(),
		End:     rng.End(),
		Message: "loop can be replaced by a map lookup",
	}
	if edits, ok := f.fix(); ok {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Replace the loop with a lookup",
			TextEdits: edits,
		}}
	}
	pass.Report(diagnostic)
}

// finder is one range statement, read as a search for one key of a map.
type finder struct {
	pass     *analysis.Pass
	file     *ast.File
	rng      *ast.RangeStmt
	key      *ast.Ident
	keyObj   types.Object
	valueVar types.Object
	ifStmt   *ast.IfStmt
	ret      *ast.ReturnStmt
	want     ast.Expr
	lookups  []*ast.IndexExpr
	reads    bool
}

// search fills the finder's fields when the range is a search for one key.
func (f *finder) search() bool {
	if f.rng.Tok != token.DEFINE {
		return false
	}
	m, ok := mapOf(f.pass, f.rng.X)
	if !ok || !noInterface(m.Key()) {
		return false
	}
	key, ok := f.rng.Key.(*ast.Ident)
	if !ok {
		return false
	}
	f.key = key
	f.keyObj = f.pass.TypesInfo.ObjectOf(f.key)
	if id, ok := f.rng.Value.(*ast.Ident); ok && id.Name != "_" {
		f.valueVar = f.pass.TypesInfo.ObjectOf(id)
	}
	return f.body() && f.condition(m.Key()) && f.returnUses()
}

// body reads the loop's body as one return, guarded by one if and nothing
// else.
func (f *finder) body() bool {
	if len(f.rng.Body.List) != 1 {
		return false
	}
	ifStmt, ok := f.rng.Body.List[0].(*ast.IfStmt)
	if !ok {
		return false
	}
	if ifStmt.Init != nil || ifStmt.Else != nil {
		return false
	}
	if len(ifStmt.Body.List) != 1 {
		return false
	}
	ret, ok := ifStmt.Body.List[0].(*ast.ReturnStmt)
	if !ok {
		return false
	}
	f.ifStmt, f.ret = ifStmt, ret
	return true
}

// condition reads the if's condition as a comparison of the key with one
// expression the rewrite can evaluate where the loop's variables are gone.
func (f *finder) condition(keyType types.Type) bool {
	bin, ok := ast.Unparen(f.ifStmt.Cond).(*ast.BinaryExpr)
	if !ok || bin.Op != token.EQL {
		return false
	}
	switch {
	case sameObject(f.pass, bin.X, f.key):
		f.want = bin.Y
	case sameObject(f.pass, bin.Y, f.key):
		f.want = bin.X
	default:
		return false
	}
	// A comparison with an interface accepts a wanted key that an index
	// rejects, so the key has to fit the map's key type.
	t := f.pass.TypesInfo.TypeOf(f.want)
	if t == nil || !types.AssignableTo(t, keyType) || !safe(f.pass, f.want) {
		return false
	}
	return !uses(f.pass, f.want, f.keyObj) && !uses(f.pass, f.want, f.valueVar)
}

// returnUses reads the return for the value the search found: the indexes that
// read the map by the key, and whether the return reads the loop's value
// variable. Every other use of the key ends the search, because an index
// expression cannot produce it.
func (f *finder) returnUses() bool {
	var unusable bool
	ast.Inspect(f.ret, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			// A literal reads the key when it runs, and nothing binds
			// it at the return.
			if uses(f.pass, n, f.keyObj) {
				unusable = true
			}
		case *ast.IndexExpr:
			index := sameObject(f.pass, n.Index, f.key)
			if sameObject(f.pass, n.X, f.rng.X) && index {
				f.lookups = append(f.lookups, n)
				f.reads = true
				return false
			}
		case *ast.Ident:
			obj := f.pass.TypesInfo.ObjectOf(n)
			if obj != nil && obj == f.keyObj {
				unusable = true
			}
			if obj != nil && obj == f.valueVar {
				f.reads = true
			}
		}
		return !unusable
	})
	return !unusable
}

// fix returns the edits that replace the range with an if that indexes the
// map, and none when the rewrite cannot stand for the loop.
func (f *finder) fix() ([]analysis.TextEdit, bool) {
	// The lookup moves to the if, so a call that runs before it in the
	// loop would run after it here.
	if last := len(f.lookups) - 1; last >= 0 {
		if precedes(f.pass, f.ret, f.lookups[last].Pos()) {
			return nil, false
		}
	}
	var value string
	if f.reads {
		value = f.valueName()
	}
	flag := pick(
		f.pass, f.rng.Pos(), value,
		"ok", "found", "present", "exists", "seen",
	)
	file, src, ok := f.source()
	if !ok {
		return nil, false
	}
	// A comment between the loop's brace and the if would end up
	// annotating the return instead of the search. One trailing the if's
	// brace stays on the header's line, where headerEnd keeps it.
	inner := span{
		lineEnd(f.pass, f.rng.Body.Lbrace+1),
		f.ifStmt.Pos(),
	}
	if commented(f.file, inner) {
		return nil, false
	}
	end := f.headerEnd(file, src)
	m, ok := render(f.rng.X)
	if !ok {
		return nil, false
	}
	if !indexed(f.rng.X) {
		m = "(" + m + ")"
	}
	// A value name equal to the map reads as a shadow of it.
	if value == m {
		return nil, false
	}
	want, ok := render(ast.Unparen(f.want))
	if !ok {
		return nil, false
	}
	head := "if _, " + flag
	if value != "" {
		head = "if " + value + ", " + flag
	}
	head += " := " + m + "[" + want + "]; " + flag + " {"
	if !f.fits(head, value, file, src) {
		return nil, false
	}
	edits := []analysis.TextEdit{{
		Pos:     f.rng.Pos(),
		End:     f.rng.Body.Lbrace + 1,
		NewText: []byte(head),
	}, {
		Pos: min(lineEnd(f.pass, f.rng.Body.Lbrace+1), f.ifStmt.Pos()),
		End: end,
	}, {
		Pos:     f.ifStmt.Body.Rbrace,
		End:     f.rng.End(),
		NewText: []byte("}"),
	}}
	for _, e := range f.lookups {
		edits = append(edits, analysis.TextEdit{
			Pos:     e.Pos(),
			End:     e.End(),
			NewText: []byte(value),
		})
	}
	spans := make([]span, len(edits))
	for i, e := range edits {
		spans[i] = span{e.Pos, e.End}
	}
	if commented(f.file, spans...) {
		return nil, false
	}
	return edits, true
}

// headerEnd returns the end of the if header: its brace when a comment trails
// it or the return sits on the same line, and otherwise the start of the next
// line, so the header's indentation does not become a blank line inside the
// new block.
func (f *finder) headerEnd(file *token.File, src []byte) token.Pos {
	var (
		end  = f.ifStmt.Body.Lbrace + 1
		tail = lineEnd(f.pass, end)
	)
	if tail > f.ret.Pos() || commented(f.file, span{end, tail}) {
		return end
	}
	// Blank lines between the header and the return go with the header, or
	// the block opens with one. A line holding a comment is not blank, so
	// the walk stops at one.
	for tail < f.ret.Pos() {
		next := lineEnd(f.pass, tail)
		if next <= tail || !blank(file, src, tail, next) {
			break
		}
		tail = next
	}
	return tail
}

// blank reports whether src[start:stop) contains nothing but space.
func blank(file *token.File, src []byte, start, stop token.Pos) bool {
	return len(bytes.TrimSpace(src[file.Offset(start):file.Offset(stop)])) == 0
}

// source returns the file the loop is written in and its text.
func (f *finder) source() (*token.File, []byte, bool) {
	file := f.pass.Fset.File(f.rng.Pos())
	if file == nil {
		return nil, nil, false
	}
	src, err := f.pass.ReadFile(file.Name())
	return file, src, err == nil
}

// fits reports whether the lines the fix writes and the lines it rewrites stay
// within the width limit: the if line at the loop's indent with any comment
// that trails it, the closing brace when the comment lands there, and each
// line a lookup becomes the value name on. A fix that overran the width would
// leave the tree worse than it found it, since the suite reports the line and
// cannot wrap it. A loop and its body on one line are measured as gofmt writes
// them, which splits the block.
func (f *finder) fits(head, value string, file *token.File, src []byte) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	var (
		start  = file.LineStart(file.Line(f.rng.Pos()))
		indent = src[file.Offset(start):file.Offset(f.rng.Pos())]
		tail   = f.trailing()
	)
	if file.Line(f.ret.Pos()) == file.Line(f.rng.Pos()) {
		// The comment ends up on the line the closing brace is on.
		if !linelen.CheckLine(slices.Concat(indent, []byte("}"), tail)) {
			return false
		}
		tail = nil
	}
	if !linelen.CheckLine(slices.Concat(indent, []byte(head), tail)) {
		return false
	}
	for _, line := range f.rewritten(value, file, src) {
		if !linelen.CheckLine(line) {
			return false
		}
	}
	return true
}

// rewritten returns every line the value name lands on, with the lookups on it
// replaced. The loop's own line is left out, because the head check measures
// it.
func (f *finder) rewritten(value string, file *token.File, src []byte) [][]byte {
	var (
		loop   = file.Line(f.rng.Pos())
		byLine = make(map[int][]*ast.IndexExpr, len(f.lookups))
	)
	for _, e := range f.lookups {
		line := file.Line(e.Pos())
		if line != loop {
			byLine[line] = append(byLine[line], e)
		}
	}
	lines := make([][]byte, 0, len(byLine))
	for line, lookups := range byLine {
		var (
			first = file.LineStart(line)
			out   []byte
			at    = file.Offset(first)
		)
		for _, e := range lookups {
			out = append(out, src[at:file.Offset(e.Pos())]...)
			out = append(out, value...)
			at = file.Offset(e.End())
		}
		stop := file.Offset(lineEnd(f.pass, lookups[len(lookups)-1].End()))
		out = append(out, src[at:stop]...)
		// The block loses a level when the loop goes, and gofmt writes
		// it that way.
		out = bytes.TrimSuffix(out, []byte("\n"))
		lines = append(lines, bytes.TrimPrefix(out, []byte("\t")))
	}
	return lines
}

// trailing returns the comments that follow the loop's opening brace on its
// first line, which the new header keeps there.
func (f *finder) trailing() (text []byte) {
	var (
		loop = f.pass.Fset.Position(f.rng.Body.Lbrace).Line
		head = f.pass.Fset.Position(f.ifStmt.Pos()).Line
	)
	for _, group := range f.file.Comments {
		if group.Pos() <= f.rng.Body.Lbrace {
			continue
		}
		line := f.pass.Fset.Position(group.Pos()).Line
		if line != loop && line != head {
			continue
		}
		for _, comment := range group.List {
			text = slices.Concat(text, []byte(" "+comment.Text))
		}
	}
	return text
}

// valueName returns the name the rewrite binds the map value to: the name the
// range declared for it, or a name nothing in scope at the loop declares.
func (f *finder) valueName() string {
	if f.valueVar != nil {
		return f.valueVar.Name()
	}
	return pick(f.pass, f.rng.Pos(), "", "v", "val", "value")
}

// pick returns the first candidate that nothing in scope at pos declares and
// that is not taken, and a numbered one when every candidate is taken.
func pick(pass *analysis.Pass, pos token.Pos, taken string, candidates ...string) string {
	scope := pass.Pkg.Scope().Innermost(pos)
	for _, name := range candidates {
		if name != taken && free(scope, pos, name) {
			return name
		}
	}
	for i := 1; ; i++ {
		name := candidates[0] + strconv.Itoa(i)
		if name != taken && free(scope, pos, name) {
			return name
		}
	}
}

// free reports whether nothing in scope at pos declares name.
func free(scope *types.Scope, pos token.Pos, name string) bool {
	_, obj := scope.LookupParent(name, pos)
	return obj == nil
}

// uses reports whether e refers to obj.
func uses(pass *analysis.Pass, e ast.Node, obj types.Object) bool {
	if obj == nil {
		return false
	}
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if pass.TypesInfo.ObjectOf(id) == obj {
				found = true
			}
		}
		return !found
	})
	return found
}

// lineEnd returns the position just past the newline that ends the line pos is
// on, so that a deletion up to it leaves no empty line behind.
func lineEnd(pass *analysis.Pass, pos token.Pos) token.Pos {
	file := pass.Fset.File(pos)
	if file == nil || file.Line(pos) == file.LineCount() {
		return pos
	}
	return file.LineStart(file.Line(pos) + 1)
}

// span is a range of source text.
type span struct {
	lo, hi token.Pos
}

// commented reports whether a comment sits in one of the spans the fix
// deletes, where it belongs to the author.
func commented(file *ast.File, spans ...span) bool {
	for _, group := range file.Comments {
		for _, s := range spans {
			if group.Pos() < s.hi && s.lo < group.End() {
				return true
			}
		}
	}
	return false
}

// mapOf returns the map type of e, and whether e has one.
func mapOf(pass *analysis.Pass, e ast.Expr) (*types.Map, bool) {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil {
		return nil, false
	}
	m, ok := t.Underlying().(*types.Map)
	return m, ok
}

// noInterface reports whether t has no interface in it, which could contain a
// value that cannot be hashed.
func noInterface(t types.Type) bool {
	switch t := types.Unalias(t).Underlying().(type) {
	case *types.Interface:
		return false
	case *types.Struct:
		for field := range t.Fields() {
			if !noInterface(field.Type()) {
				return false
			}
		}
	case *types.Array:
		return noInterface(t.Elem())
	}
	return true
}

// safe reports whether evaluating e can run code or panic. A range evaluates
// the wanted key only for an entry it visits, and the index evaluates it
// always.
func safe(pass *analysis.Pass, e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Ident, *ast.BasicLit:
		return true
	case *ast.ParenExpr:
		return safe(pass, e.X)
	case *ast.CompositeLit:
		for _, elt := range e.Elts {
			if !safe(pass, elt) {
				return false
			}
		}
		return true
	case *ast.KeyValueExpr:
		return safe(pass, e.Key) && safe(pass, e.Value)
	case *ast.SelectorExpr:
		return follows(pass, e) && safe(pass, e.X)
	case *ast.IndexExpr:
		// An index into a map cannot panic; one into a slice or a
		// string checks its bounds.
		m, ok := mapOf(pass, e.X)
		return ok && noInterface(m.Key()) && safe(pass, e.X) &&
			safe(pass, e.Index)
	case *ast.UnaryExpr:
		return e.Op != token.ARROW && safe(pass, e.X)
	case *ast.CallExpr:
		return called(pass, e)
	case *ast.BinaryExpr:
		return computed(pass, e)
	}
	return false
}

// called reports whether a call cannot run user code or panic: a conversion
// judged on what it converts, or len, cap, min or max, with its arguments held
// to the same rule.
func called(pass *analysis.Pass, e *ast.CallExpr) bool {
	if pass.TypesInfo.Types[e.Fun].IsType() {
		return converted(pass, e)
	}
	id, ok := ast.Unparen(e.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[id].(*types.Builtin)
	if !ok {
		return false
	}
	switch builtin.Name() {
	case "len", "cap", "min", "max":
	default:
		return false
	}
	for _, arg := range e.Args {
		if !safe(pass, arg) {
			return false
		}
	}
	return true
}

// converted reports whether a conversion cannot panic: nothing it converts
// can, and it does not build an array.
func converted(pass *analysis.Pass, e *ast.CallExpr) bool {
	if t := pass.TypesInfo.TypeOf(e); t == nil || !arrayless(t) {
		return false
	}
	for _, arg := range e.Args {
		if !safe(pass, arg) {
			return false
		}
	}
	return true
}

// arrayless reports whether converting a slice to t cannot panic: a conversion
// to an array, or to a pointer to one, panics when the slice is shorter.
func arrayless(t types.Type) bool {
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	u := t.Underlying()
	if p, ok := u.(*types.Pointer); ok {
		u = p.Elem().Underlying()
	}
	_, array := u.(*types.Array)
	return !array
}

// follows reports whether reading e dereferences no pointer: neither the value
// it starts from, nor a pointer-typed field on the way to the one selected.
func follows(pass *analysis.Pass, e *ast.SelectorExpr) bool {
	t := pass.TypesInfo.TypeOf(e.X)
	if t == nil {
		return false
	}
	if _, ok := t.Underlying().(*types.Pointer); ok {
		return false
	}
	sel, ok := pass.TypesInfo.Selections[e]
	if !ok {
		return true
	}
	t = sel.Recv()
	index := sel.Index()
	for _, i := range index[:len(index)-1] {
		st, ok := t.Underlying().(*types.Struct)
		if !ok {
			return false
		}
		field := st.Field(i)
		if _, ok := field.Type().Underlying().(*types.Pointer); ok {
			return false
		}
		t = field.Type()
	}
	return true
}

// computed reports whether a binary expression cannot panic.
func computed(pass *analysis.Pass, e *ast.BinaryExpr) bool {
	switch e.Op {
	case token.QUO, token.REM, token.SHL, token.SHR:
		// A zero divisor and a negative shift count both panic.
		return false
	}
	if e.Op == token.EQL || e.Op == token.NEQ {
		// Two interface values panic on comparison when both hold the
		// same type and that type cannot be hashed.
		if t := pass.TypesInfo.TypeOf(e.X); t == nil || !noInterface(t) {
			return false
		}
	}
	return safe(pass, e.X) && safe(pass, e.Y)
}

// indexed reports whether e can stand before an index without parentheses.
func indexed(e ast.Expr) bool {
	switch e.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.TypeAssertExpr,
		*ast.ParenExpr, *ast.CallExpr, *ast.CompositeLit:
		return true
	}
	return false
}

// sameObject reports whether a and b refer to the same object, so a shadowed
// name is a different value.
func sameObject(pass *analysis.Pass, a, b ast.Expr) bool {
	switch x := ast.Unparen(a).(type) {
	case *ast.Ident:
		y, ok := ast.Unparen(b).(*ast.Ident)
		return ok && pass.TypesInfo.ObjectOf(x) == pass.TypesInfo.ObjectOf(y)
	case *ast.StarExpr:
		y, ok := ast.Unparen(b).(*ast.StarExpr)
		return ok && sameObject(pass, x.X, y.X)
	case *ast.SelectorExpr:
		y, ok := ast.Unparen(b).(*ast.SelectorExpr)
		if !ok {
			return false
		}
		sel := pass.TypesInfo.ObjectOf(x.Sel)
		return sel != nil && sel == pass.TypesInfo.ObjectOf(y.Sel) &&
			sameObject(pass, x.X, y.X)
	case *ast.IndexExpr:
		y, ok := ast.Unparen(b).(*ast.IndexExpr)
		return ok && sameObject(pass, x.X, y.X) &&
			sameObject(pass, x.Index, y.Index)
	case *ast.BasicLit:
		// A map held in a slice is named by a literal index.
		y, ok := ast.Unparen(b).(*ast.BasicLit)
		return ok && x.Kind == y.Kind && x.Value == y.Value
	}
	return false
}

// render returns e as source text on one line, and fails when e has no
// one-line form, as a raw string has none. An empty FileSet makes the printer
// place the expression itself, dropping the author's line breaks.
func render(e ast.Expr) (string, bool) {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), e); err != nil {
		return "", false
	}
	if bytes.ContainsRune(buf.Bytes(), '\n') {
		return "", false
	}
	return buf.String(), true
}

// precedes reports whether a call or a channel receive in node finishes before
// pos starts, so that the loop runs it before the read the fix moves into the
// if.
func precedes(pass *analysis.Pass, node ast.Node, pos token.Pos) bool {
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			// A conversion is not a call, and a builtin cannot touch
			// the map.
			tv := pass.TypesInfo.Types[n.Fun]
			if !tv.IsType() && !tv.IsBuiltin() && n.End() < pos {
				found = true
			}
		case *ast.UnaryExpr:
			if n.Op == token.ARROW && n.End() < pos {
				found = true
			}
		}
		return !found
	})
	return found
}
