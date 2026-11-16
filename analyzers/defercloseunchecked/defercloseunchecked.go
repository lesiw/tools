// Package defercloseunchecked provides an analysis.Analyzer that reports a
// deferred call to Close or Flush whose error is discarded.
//
// A Close can return an error, and a Write can return before its data reaches
// the underlying writer because the data is still buffered.
// bufio.Writer.Flush "writes any buffered data to the underlying [io.Writer]",
// and gzip.Writer.Close flushes "any unwritten data to the underlying
// io.Writer" and writes the GZIP footer. The Go spec's Defer statements
// section says "If the deferred function has any return values, they are
// discarded when the function completes", so the error is lost. The same
// section evaluates the function value and parameters when the defer statement
// executes. A method value saves its receiver: "The expression x is evaluated
// and saved during the evaluation of the method value; the saved copy is then
// used as the receiver in any calls." So the defer statement captures the
// receiver. A write after the defer statement counts until the variable is
// assigned another file, because the deferred Close still uses the saved
// receiver.
//
// A Close is reported on an os.File opened for writing, or on a writer with a
// Flush method that returns an error. A Close on an *os.File is reported when
// the file came from os.Create or os.CreateTemp, from an os.OpenFile call with
// O_WRONLY or O_RDWR, from os.Stdout or os.Stderr, or when its origin is
// unknown and the function writes through it. An os.Create call opens the
// file for reading and writing, and an os.OpenFile call sets an access mode
// only from O_RDONLY, O_WRONLY, or O_RDWR. A file from os.Open or os.Pipe is
// left alone. A Close on a net.Conn or io.PipeWriter is left alone because
// its static type has no Flush method. A Flush is reported when it returns an
// error.
//
// A file is followed through local identifiers, conversions, type assertions,
// dereferences, address-of expressions, and a one-element literal. A file
// whose origin cannot be determined is left alone when its static type is
// os.File or holds one, as with a file from a helper or a package variable,
// unless the function writes through the same identifier. An
// interface value is judged by its origin and the methods its static type has.
// A file whose last assignment before the defer is on a conditional path is
// left alone, unless the function writes through it. A write is tracked on the
// identifier it names, so a write through one alias does not mark another.
// Test files are not checked; a close error there is a test failure at most.
//
// The rule is adapted from gosec's G307, which reports a deferred method that
// returns an error. This rule covers Close and Flush, and requires a Close to
// have a writer as its receiver. gosec no longer reports it; G307 now names an
// os.Create permission check.
//
// Report only: reading the error means replacing the defer with a call near
// the end of the function or with a func literal that reads it; the repair
// differs with the surrounding code.
package defercloseunchecked

import (
	"cmp"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "defercloseunchecked",
	Doc:  "reports a deferred Close or Flush that discards its error",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	var diags []analysis.Diagnostic
	for _, file := range pass.Files {
		name := pass.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Body != nil {
					diags = append(diags, check(pass, n.Body)...)
				}
			case *ast.FuncLit:
				diags = append(diags, check(pass, n.Body)...)
			}
			return true
		})
	}
	slices.SortFunc(diags, func(a, b analysis.Diagnostic) int {
		return cmp.Compare(a.Pos, b.Pos)
	})
	for _, diag := range diags {
		pass.Report(diag)
	}
	return nil, nil
}

// check returns the diagnostics for the defers in body. A nested function
// literal is checked on its own, so a write in one literal does not count as
// a write in another.
func check(pass *analysis.Pass, body *ast.BlockStmt) []analysis.Diagnostic {
	var (
		c = checker{
			pass:        pass,
			writable:    map[types.Object]bool{},
			writes:      map[types.Object][]token.Pos{},
			deferWrites: map[types.Object][]token.Pos{},
			assigns:     map[types.Object][]token.Pos{},
			history:     map[types.Object][]assignment{},
		}
		diags []analysis.Diagnostic
	)
	collect(pass, body, c.writes, c.deferWrites, c.assigns)
	c.branches = branchSpans(pass, body)
	c.exits = exitSpans(pass, body)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit, *ast.GoStmt:
			return false
		case *ast.DeferStmt:
			if diag, ok := c.diagnostic(n); ok {
				diags = append(diags, diag)
			}
			return false
		case *ast.AssignStmt:
			noteAssign(pass, n, c.writable, c.history)
		case *ast.ValueSpec:
			noteValue(pass, n, c.writable, c.history)
		case *ast.RangeStmt:
			noteRange(pass, n, c.writable, c.history)
		}
		return true
	})
	return diags
}

// checker holds the writes and file origins of one function body.
type checker struct {
	pass        *analysis.Pass
	writable    map[types.Object]bool
	writes      map[types.Object][]token.Pos
	deferWrites map[types.Object][]token.Pos
	assigns     map[types.Object][]token.Pos
	history     map[types.Object][]assignment
	branches    []span
	exits       []span
}

// assignment is one file origin recorded for an object.
type assignment struct {
	value bool
	known bool
	pos   token.Pos
}

// diagnostic returns the report for stmt, if its deferred call discards an
// error.
func (c *checker) diagnostic(stmt *ast.DeferStmt) (analysis.Diagnostic, bool) {
	sel, ok := ast.Unparen(stmt.Call.Fun).(*ast.SelectorExpr)
	if !ok || !c.unchecked(stmt.Call, sel, stmt.Pos()) {
		return analysis.Diagnostic{}, false
	}
	return analysis.Diagnostic{
		Pos:     stmt.Call.Pos(),
		End:     stmt.Call.End(),
		Message: "deferred " + sel.Sel.Name + " discards its error",
	}, true
}

// unchecked reports whether sel names a Close on a writer or a Flush, and the
// method it names returns an error.
func (c *checker) unchecked(call *ast.CallExpr, sel *ast.SelectorExpr, pos token.Pos) bool {
	name := sel.Sel.Name
	if name != "Close" && name != "Flush" {
		return false
	}
	if _, ok := methodSelection(c.pass, sel); !ok {
		return false
	}
	if name == "Close" {
		var (
			recv = methodReceiver(c.pass, call, sel)
			typ  = c.pass.TypesInfo.TypeOf(recv)
		)
		recv = ast.Unparen(recv)
		addr := addressable(c.pass, recv)
		if fileLike(typ) {
			if !closesFile(c.pass, sel) {
				return false
			}
			value, known := c.fileState(recv, pos, true)
			if !known || !value {
				return false
			}
		} else {
			value, known := c.fileState(recv, pos, false)
			switch {
			case known && !value:
				return false
			case known:
			case !writes(typ, addr) || !flushes(typ, addr):
				return false
			}
		}
	}
	typ := c.pass.TypesInfo.TypeOf(sel)
	if typ == nil {
		return false
	}
	sig, ok := typ.Underlying().(*types.Signature)
	return ok && returnsError(sig)
}

// addressable reports whether expr can be the receiver of a pointer method.
func addressable(pass *analysis.Pass, expr ast.Expr) bool {
	if pass.TypesInfo.Types[expr].Addressable() {
		return true
	}
	_, ok := expr.(*ast.StarExpr)
	return ok
}

// returnsError reports whether a result of sig is assignable to error.
func returnsError(sig *types.Signature) bool {
	err := types.Universe.Lookup("error").Type()
	for result := range sig.Results().Variables() {
		if types.AssignableTo(result.Type(), err) {
			return true
		}
	}
	return false
}

// fileState returns whether expr names a file opened for writing and whether
// the origin is known. An unknown origin with a write through expr before or
// after the defer is writable. A known read-only origin stays read-only.
func (c *checker) fileState(expr ast.Expr, pos token.Pos, allowWrite bool) (value, known bool) {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		if conversion(c.pass, expr) {
			return c.fileState(expr.Args[0], pos, allowWrite)
		}
	case *ast.TypeAssertExpr:
		return c.fileState(expr.X, pos, allowWrite)
	case *ast.CompositeLit:
		if len(expr.Elts) == 1 {
			elt := expr.Elts[0]
			if keyed, ok := elt.(*ast.KeyValueExpr); ok {
				elt = keyed.Value
			}
			return c.fileState(elt, pos, allowWrite)
		}
	case *ast.StarExpr:
		return c.fileState(expr.X, pos, allowWrite)
	case *ast.UnaryExpr:
		if expr.Op == token.AND {
			return c.fileState(expr.X, pos, allowWrite)
		}
	case *ast.Ident:
		obj := c.pass.TypesInfo.ObjectOf(expr)
		if obj == nil {
			return false, false
		}
		if value, known := c.origin(obj, pos); known {
			return value, true
		}
		if !allowWrite {
			return false, false
		}
		if c.writeBefore(obj, pos) || c.afterWrite(obj, pos) {
			return true, true
		}
		return false, false
	}
	if isStdFile(c.pass, expr) {
		return true, true
	}
	return false, false
}

// origin returns the origin recorded for obj before the defer, or unknown
// when the last assignment is conditional or has no known origin.
func (c *checker) origin(obj types.Object, pos token.Pos) (value, known bool) {
	history := c.history[obj]
	if len(history) == 0 {
		return false, false
	}
	last := history[len(history)-1]
	if !last.known || conditionalFor(c.branches, last.pos, pos) {
		return false, false
	}
	return last.value, true
}

// writeBefore reports whether obj has a write before pos that no assignment
// has replaced.
func (c *checker) writeBefore(obj types.Object, pos token.Pos) bool {
	last := token.NoPos
	for _, assign := range c.assigns[obj] {
		if assign < pos && assign > last {
			if !conditionalFor(c.branches, assign, pos) {
				last = assign
			}
		}
	}
	for _, write := range c.writes[obj] {
		if write >= pos || write < last {
			continue
		}
		if reachesBefore(c.branches, write, pos) {
			return true
		}
	}
	return false
}

// afterWrite reports whether obj has a write after pos and before the next
// assignment to obj.
func (c *checker) afterWrite(obj types.Object, pos token.Pos) bool {
	for _, write := range c.writes[obj] {
		if write <= pos || exitsBeforeReturn(c.exits, write) {
			continue
		}
		if deferUnreachable(c.branches, pos, write) {
			continue
		}
		if blocked(c.branches, c.assigns[obj], pos, write) {
			continue
		}
		if !mutuallyExclusive(c.branches, write, pos) {
			return true
		}
	}
	for _, write := range c.deferWrites[obj] {
		if write <= pos || exitsBeforeReturn(c.exits, write) {
			continue
		}
		if deferUnreachable(c.branches, pos, write) {
			continue
		}
		if deferredReaches(c.branches, c.assigns[obj], pos, write) {
			return true
		}
	}
	return false
}

// deferredReaches reports whether a deferred write reaches the Close.
func deferredReaches(spans []span, assigns []token.Pos, pos, write token.Pos) bool {
	return !blocked(spans, assigns, pos, write) &&
		!mutuallyExclusive(spans, write, pos)
}

// blocked reports whether an assignment between pos and write replaces obj.
func blocked(spans []span, assigns []token.Pos, pos, write token.Pos) bool {
	for _, assign := range assigns {
		if assign > pos && assign < write {
			if !conditionalFor(spans, assign, write) {
				return true
			}
		}
	}
	return false
}

// isOSFile reports whether typ is *os.File.
func isOSFile(typ types.Type) bool {
	if typ == nil {
		return false
	}
	typ = types.Unalias(typ)
	if ptr, ok := typ.Underlying().(*types.Pointer); ok {
		typ = types.Unalias(ptr.Elem())
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "os" && named.Obj().Name() == "File"
}

// fileLike reports whether typ is an os.File or holds one in a field.
func fileLike(typ types.Type) bool {
	return holdsFile(typ, map[types.Type]bool{})
}

// holdsFile reports whether typ is an os.File or holds one in a field.
func holdsFile(typ types.Type, seen map[types.Type]bool) bool {
	if isOSFile(typ) {
		return true
	}
	typ = types.Unalias(typ)
	if seen[typ] {
		return false
	}
	seen[typ] = true
	switch typ := typ.Underlying().(type) {
	case *types.Pointer:
		return holdsFile(typ.Elem(), seen)
	case *types.Struct:
		for field := range typ.Fields() {
			if holdsFile(field.Type(), seen) {
				return true
			}
		}
	}
	return false
}

// isStdFile reports whether expr is os.Stdout or os.Stderr.
func isStdFile(pass *analysis.Pass, expr ast.Expr) bool {
	sel, ok := ast.Unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	v, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Var)
	if !ok || v.Pkg() == nil || v.Pkg().Path() != "os" {
		return false
	}
	return v.Name() == "Stdout" || v.Name() == "Stderr"
}

// noteRange records that each name assigned by a range header has an unknown
// origin.
func noteRange(pass *analysis.Pass, stmt *ast.RangeStmt, writable map[types.Object]bool, history map[types.Object][]assignment) {
	exprs := []ast.Expr{stmt.Key, stmt.Value}
	for _, expr := range exprs {
		if expr == nil {
			continue
		}
		id, ok := ast.Unparen(expr).(*ast.Ident)
		if !ok {
			continue
		}
		obj := pass.TypesInfo.ObjectOf(id)
		if obj == nil {
			continue
		}
		writable[obj] = false
		history[obj] = append(
			history[obj],
			assignment{false, false, stmt.Pos()},
		)
	}
}

// noteAssign records whether each os.File assigned by stmt was opened for
// writing.
func noteAssign(pass *analysis.Pass, stmt *ast.AssignStmt, writable map[types.Object]bool, history map[types.Object][]assignment) {
	if len(stmt.Rhs) == 1 {
		noteOrigins(
			pass, stmt.Lhs, stmt.Rhs[0],
			writable, history, stmt.Pos(),
		)
		return
	}
	if len(stmt.Rhs) != len(stmt.Lhs) {
		return
	}
	origins := make([]origin, len(stmt.Rhs))
	for i := range stmt.Rhs {
		origins[i].value, origins[i].known = writability(
			pass, stmt.Rhs[i],
			maps.Clone(writable),
		)
	}
	for i := range stmt.Rhs {
		applyOrigins(
			pass, stmt.Lhs[i:i+1], stmt.Rhs[i], origins[i],
			writable, history, stmt.Pos(),
		)
	}
}

// noteValue records whether each os.File declared by spec was opened for
// writing.
func noteValue(pass *analysis.Pass, spec *ast.ValueSpec, writable map[types.Object]bool, history map[types.Object][]assignment) {
	lhs := make([]ast.Expr, 0, len(spec.Names))
	for _, name := range spec.Names {
		lhs = append(lhs, name)
	}
	if len(spec.Values) == 1 {
		noteOrigins(
			pass, lhs, spec.Values[0],
			writable, history, spec.Pos(),
		)
		return
	}
	if len(spec.Values) != len(spec.Names) {
		return
	}
	origins := make([]origin, len(spec.Values))
	for i := range spec.Values {
		origins[i].value, origins[i].known = writability(
			pass, spec.Values[i],
			maps.Clone(writable),
		)
	}
	for i := range spec.Values {
		applyOrigins(
			pass, lhs[i:i+1], spec.Values[i], origins[i],
			writable, history, spec.Pos(),
		)
	}
}

// origin is a file origin the rule has determined.
type origin struct {
	value bool
	known bool
}

// noteOrigins records the writability of one expression in each name in lhs.
func noteOrigins(pass *analysis.Pass, lhs []ast.Expr, rhs ast.Expr, writable map[types.Object]bool, history map[types.Object][]assignment, pos token.Pos) {
	value, known := writability(pass, rhs, writable)
	applyOrigins(
		pass, lhs, rhs, origin{value, known},
		writable, history, pos,
	)
}

// applyOrigins records one origin in each name in lhs.
func applyOrigins(pass *analysis.Pass, lhs []ast.Expr, rhs ast.Expr, origin origin, writable map[types.Object]bool, history map[types.Object][]assignment, pos token.Pos) {
	var (
		value, known = origin.value, origin.known
		rhsFile      = isOSFile(pass.TypesInfo.TypeOf(rhs))
	)
	for _, expr := range lhs {
		id, ok := ast.Unparen(expr).(*ast.Ident)
		if !ok {
			continue
		}
		obj := pass.TypesInfo.ObjectOf(id)
		if obj == nil {
			continue
		}
		_, tracked := writable[obj]
		if !known && !rhsFile && !tracked {
			if !isOSFile(pass.TypesInfo.TypeOf(id)) {
				continue
			}
		}
		writable[obj] = value
		history[obj] = append(history[obj], assignment{value, known, pos})
	}
}

// writability reports whether rhs names an os.File opened for writing, and
// whether the origin is known.
func writability(pass *analysis.Pass, rhs ast.Expr, writable map[types.Object]bool) (value, known bool) {
	switch rhs := ast.Unparen(rhs).(type) {
	case *ast.CallExpr:
		return callWritability(pass, rhs, writable)
	case *ast.CompositeLit:
		return literalWritability(pass, rhs, writable)
	case *ast.SelectorExpr:
		return nameWritability(pass, rhs)
	case *ast.TypeAssertExpr:
		return writability(pass, rhs.X, writable)
	case *ast.StarExpr:
		return writability(pass, rhs.X, writable)
	case *ast.UnaryExpr:
		if rhs.Op == token.AND {
			return writability(pass, rhs.X, writable)
		}
	case *ast.Ident:
		if obj := pass.TypesInfo.ObjectOf(rhs); obj != nil {
			value, known := writable[obj]
			return value, known
		}
	}
	return false, false
}

// callWritability reports whether a call opens a file for writing.
func callWritability(pass *analysis.Pass, call *ast.CallExpr, writable map[types.Object]bool) (value, known bool) {
	if conversion(pass, call) {
		return writability(pass, call.Args[0], writable)
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "os" {
		return false, false
	}
	switch fn.Name() {
	case "Create", "CreateTemp":
		return true, true
	case "Open", "Pipe":
		return false, true
	case "OpenFile":
		if len(call.Args) < 2 {
			return false, false
		}
		return writeFlags(pass, call.Args[1])
	}
	return false, false
}

// conversion reports whether call is a type conversion with one argument.
func conversion(pass *analysis.Pass, call *ast.CallExpr) bool {
	tv, ok := pass.TypesInfo.Types[call.Fun]
	return ok && tv.IsType() && len(call.Args) == 1
}

// literalWritability reports whether a one-element literal holds a file opened
// for writing.
func literalWritability(pass *analysis.Pass, lit *ast.CompositeLit, writable map[types.Object]bool) (value, known bool) {
	if len(lit.Elts) != 1 {
		return false, false
	}
	elt := lit.Elts[0]
	if keyed, ok := elt.(*ast.KeyValueExpr); ok {
		elt = keyed.Value
	}
	return writability(pass, elt, writable)
}

// nameWritability reports whether a name from the os package is a standard
// output file.
func nameWritability(pass *analysis.Pass, sel *ast.SelectorExpr) (value, known bool) {
	v, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Var)
	if !ok || v.Pkg() == nil || v.Pkg().Path() != "os" {
		return false, false
	}
	switch v.Name() {
	case "Stdout", "Stderr":
		return true, true
	case "Stdin":
		return false, true
	}
	return false, false
}

// writeFlags reports whether expr names a flag value that allows a write, and
// whether the value is known. O_WRONLY is 1 and O_RDWR is 2 in the os package.
func writeFlags(pass *analysis.Pass, expr ast.Expr) (value, known bool) {
	exprValue := pass.TypesInfo.Types[expr].Value
	if exprValue == nil {
		return false, false
	}
	flags, ok := constant.Int64Val(exprValue)
	if !ok {
		return false, false
	}
	return flags&3 != 0, true
}

// span is a range of source positions.
type span struct {
	start      token.Pos
	end        token.Pos
	group      int
	run        int
	terminates bool
	exits      bool
}

// branchSpans returns the bodies that run on one path only. A file assigned
// in one may not have that origin when the defer runs.
func branchSpans(pass *analysis.Pass, body *ast.BlockStmt) (spans []span) {
	var (
		group       int
		clauseGroup = map[token.Pos]int{}
		clauseRun   = map[token.Pos]int{}
	)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.SwitchStmt:
			group++
			sameGroup(n.Body.List, group, clauseGroup, clauseRun)
		case *ast.TypeSwitchStmt:
			group++
			sameGroup(n.Body.List, group, clauseGroup, clauseRun)
		case *ast.SelectStmt:
			group++
			sameGroup(n.Body.List, group, clauseGroup, clauseRun)
		}
		return true
	})
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.IfStmt:
			group++
			spans = append(spans, span{
				n.Body.Pos(), n.Body.End(), group, 0,
				blockTerminates(pass, n.Body),
				blockExits(pass, n.Body),
			})
			if n.Else != nil {
				spans = append(spans, span{
					n.Else.Pos(), n.Else.End(), group, 1,
					stmtTerminates(pass, n.Else),
					stmtExits(pass, n.Else),
				})
			}
		case *ast.ForStmt:
			spans = append(spans, span{
				n.Body.Pos(), n.Body.End(), 0, 0,
				blockTerminates(pass, n.Body),
				blockExits(pass, n.Body),
			})
			if n.Post != nil {
				spans = append(spans, span{
					n.Post.Pos(), n.Post.End(),
					0, 0, false, false,
				})
			}
		case *ast.RangeStmt:
			spans = append(spans, span{
				n.Pos(), n.End(), 0, 0,
				blockTerminates(pass, n.Body),
				blockExits(pass, n.Body),
			})
		case *ast.CaseClause:
			spans = append(spans, span{
				n.Pos(), n.End(), clauseGroup[n.Pos()],
				clauseRun[n.Pos()],
				listTerminates(pass, n.Body),
				listExits(pass, n.Body),
			})
		case *ast.CommClause:
			spans = append(spans, span{
				n.Pos(), n.End(), clauseGroup[n.Pos()],
				clauseRun[n.Pos()],
				listTerminates(pass, n.Body),
				listExits(pass, n.Body),
			})
		}
		return true
	})
	return spans
}

// sameGroup records group and run for each clause in list. A fallthrough
// keeps the next clause in the same run.
func sameGroup(list []ast.Stmt, group int, groups, runs map[token.Pos]int) {
	var run int
	for _, stmt := range list {
		var body []ast.Stmt
		switch stmt := stmt.(type) {
		case *ast.CaseClause:
			body = stmt.Body
		case *ast.CommClause:
			body = stmt.Body
		}
		groups[stmt.Pos()] = group
		runs[stmt.Pos()] = run
		if !endsFallthrough(body) {
			run++
		}
	}
}

// endsBreak reports whether list ends in a break.
func endsBreak(list []ast.Stmt) bool {
	if len(list) == 0 {
		return false
	}
	branch, ok := list[len(list)-1].(*ast.BranchStmt)
	return ok && branch.Tok == token.BREAK
}

// endsFallthrough reports whether list ends in a fallthrough.
func endsFallthrough(list []ast.Stmt) bool {
	if len(list) == 0 {
		return false
	}
	branch, ok := list[len(list)-1].(*ast.BranchStmt)
	return ok && branch.Tok == token.FALLTHROUGH
}

// conditionalFor reports whether assign runs on a path that defer does not,
// so the assignment may not have run when the defer statement executes.
func conditionalFor(spans []span, assign, deferPos token.Pos) bool {
	for _, s := range spans {
		if contains(s, assign) && !contains(s, deferPos) {
			return true
		}
	}
	return false
}

// reachesBefore reports whether a write before defer can reach it.
func reachesBefore(spans []span, write, deferPos token.Pos) bool {
	for _, s := range spans {
		if contains(s, write) && !contains(s, deferPos) && s.terminates {
			return false
		}
	}
	return !mutuallyExclusive(spans, write, deferPos)
}

// mutuallyExclusive reports whether a and b are in different branches of
// one conditional.
func mutuallyExclusive(spans []span, a, b token.Pos) bool {
	for _, first := range spans {
		if first.group == 0 || !contains(first, a) {
			continue
		}
		for _, second := range spans {
			if sameBranch(first, second) && contains(second, b) {
				if !contains(second, a) {
					return true
				}
			}
		}
	}
	return false
}

// sameBranch reports whether a and b are alternatives in one conditional.
func sameBranch(a, b span) bool {
	return a.group == b.group && a.run != b.run
}

// contains reports whether pos falls in s.
func contains(s span, pos token.Pos) bool {
	return pos >= s.start && pos < s.end
}

// blockTerminates reports whether block ends in a statement that leaves it.
func blockTerminates(pass *analysis.Pass, block *ast.BlockStmt) bool {
	return listTerminates(pass, block.List)
}

// listTerminates reports whether list ends in a statement that leaves it.
func listTerminates(pass *analysis.Pass, list []ast.Stmt) bool {
	if len(list) == 0 {
		return false
	}
	return stmtTerminates(pass, list[len(list)-1])
}

// stmtTerminates reports whether stmt leaves its block before the next one.
func stmtTerminates(pass *analysis.Pass, stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return stmt.Tok == token.BREAK || stmt.Tok == token.GOTO
	case *ast.BlockStmt:
		return blockTerminates(pass, stmt)
	case *ast.IfStmt:
		return blockTerminates(pass, stmt.Body) && stmt.Else != nil &&
			stmtTerminates(pass, stmt.Else)
	case *ast.ExprStmt:
		return stopsProgram(pass, stmt.X)
	case *ast.SwitchStmt:
		return switchTerminates(pass, stmt.Body.List)
	case *ast.TypeSwitchStmt:
		return switchTerminates(pass, stmt.Body.List)
	case *ast.SelectStmt:
		return clausesTerminate(pass, stmt.Body.List, true)
	}
	return false
}

// blockExits reports whether block ends in a call that skips defers.
func blockExits(pass *analysis.Pass, block *ast.BlockStmt) bool {
	return listExits(pass, block.List)
}

// listExits reports whether list ends in a call that skips defers.
func listExits(pass *analysis.Pass, list []ast.Stmt) bool {
	if len(list) == 0 {
		return false
	}
	return stmtExits(pass, list[len(list)-1])
}

// stmtExits reports whether stmt ends in a call that skips defers.
func stmtExits(pass *analysis.Pass, stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.BlockStmt:
		return blockExits(pass, stmt)
	case *ast.IfStmt:
		return blockExits(pass, stmt.Body) && stmt.Else != nil &&
			stmtExits(pass, stmt.Else)
	case *ast.ExprStmt:
		return exitsProgram(pass, stmt.X)
	case *ast.SwitchStmt:
		return switchExits(pass, stmt.Body.List)
	case *ast.TypeSwitchStmt:
		return switchExits(pass, stmt.Body.List)
	case *ast.SelectStmt:
		return clausesExit(pass, stmt.Body.List, true)
	}
	return false
}

// switchTerminates reports whether a switch ends every path.
func switchTerminates(pass *analysis.Pass, list []ast.Stmt) bool {
	return clausesTerminate(pass, list, hasDefault(list))
}

// switchExits reports whether a switch exits on every path.
func switchExits(pass *analysis.Pass, list []ast.Stmt) bool {
	return clausesExit(pass, list, hasDefault(list))
}

// clausesTerminate reports whether every clause body terminates, and the
// construct runs one of them.
func clausesTerminate(pass *analysis.Pass, list []ast.Stmt, always bool) bool {
	if !always {
		return false
	}
	for _, stmt := range list {
		var body []ast.Stmt
		switch stmt := stmt.(type) {
		case *ast.CaseClause:
			body = stmt.Body
		case *ast.CommClause:
			body = stmt.Body
		}
		if endsFallthrough(body) {
			continue
		}
		if endsBreak(body) || !listTerminates(pass, body) {
			return false
		}
	}
	return true
}

// clausesExit reports whether every clause body exits, and the construct runs
// one of them.
func clausesExit(pass *analysis.Pass, list []ast.Stmt, always bool) bool {
	if !always {
		return false
	}
	for _, stmt := range list {
		var body []ast.Stmt
		switch stmt := stmt.(type) {
		case *ast.CaseClause:
			body = stmt.Body
		case *ast.CommClause:
			body = stmt.Body
		}
		if endsFallthrough(body) {
			continue
		}
		if endsBreak(body) || !listExits(pass, body) {
			return false
		}
	}
	return true
}

// hasDefault reports whether list holds a default clause.
func hasDefault(list []ast.Stmt) bool {
	for _, stmt := range list {
		switch stmt := stmt.(type) {
		case *ast.CaseClause:
			if len(stmt.List) == 0 {
				return true
			}
		case *ast.CommClause:
			if stmt.Comm == nil {
				return true
			}
		}
	}
	return false
}

// stopsProgram reports whether expr ends the program.
func stopsProgram(pass *analysis.Pass, expr ast.Expr) bool {
	if exitsProgram(pass, expr) {
		return true
	}
	if panicsProgram(pass, expr) || goexitsProgram(pass, expr) {
		return true
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("panic")
}

// goexitsProgram reports whether expr ends the goroutine without falling
// through. Goexit still runs deferred calls.
func goexitsProgram(pass *analysis.Pass, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == "runtime" &&
		fn.Name() == "Goexit"
}

// panicsProgram reports whether expr calls a log function that panics.
func panicsProgram(pass *analysis.Pass, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "log" {
		return false
	}
	switch fn.Name() {
	case "Panic", "Panicf", "Panicln":
		return true
	}
	return false
}

// exitsProgram reports whether expr exits without running deferred calls.
func exitsProgram(pass *analysis.Pass, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil {
		return false
	}
	switch fn.Pkg().Path() {
	case "os", "syscall":
		return fn.Name() == "Exit"
	case "log":
		switch fn.Name() {
		case "Fatal", "Fatalf", "Fatalln":
			return true
		}
	}
	return false
}

// exitSpans returns the blocks that exit without running deferred calls.
func exitSpans(pass *analysis.Pass, body *ast.BlockStmt) (spans []span) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BlockStmt:
			if blockExits(pass, n) {
				spans = append(spans, span{
					n.Pos(), n.End(), 0, 0, false, true,
				})
			}
		case *ast.CaseClause:
			if listExits(pass, n.Body) {
				spans = append(spans, span{
					n.Pos(), n.End(), 0, 0, false, true,
				})
			}
		case *ast.CommClause:
			if listExits(pass, n.Body) {
				spans = append(spans, span{
					n.Pos(), n.End(), 0, 0, false, true,
				})
			}
		}
		return true
	})
	return spans
}

// deferUnreachable reports whether deferPos is in a branch that leaves before
// writePos.
func deferUnreachable(spans []span, deferPos, writePos token.Pos) bool {
	for _, s := range spans {
		if s.terminates && contains(s, deferPos) && !contains(s, writePos) {
			return true
		}
	}
	return false
}

// exitsBeforeReturn reports whether pos is in a branch that exits first.
func exitsBeforeReturn(spans []span, pos token.Pos) bool {
	for _, s := range spans {
		if s.exits && contains(s, pos) {
			return true
		}
	}
	return false
}

// collect records the positions of writes and assignments for use after a
// defer statement.
func collect(pass *analysis.Pass, body *ast.BlockStmt, writes, deferWrites, assigns map[types.Object][]token.Pos) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit, *ast.GoStmt:
			return false
		case *ast.DeferStmt:
			if obj, ok := writeObject(pass, n.Call); ok {
				deferWrites[obj] = append(deferWrites[obj], n.Pos())
			}
			if lit, ok := ast.Unparen(n.Call.Fun).(*ast.FuncLit); ok {
				collectCalls(pass, lit.Body, deferWrites)
			}
			for _, arg := range n.Call.Args {
				collectCalls(pass, arg, writes)
			}
			return false
		case *ast.AssignStmt:
			for _, expr := range n.Lhs {
				recordObject(pass, expr, n.Pos(), assigns)
			}
		case *ast.ValueSpec:
			for _, name := range n.Names {
				recordObject(pass, name, n.Pos(), assigns)
			}
		case *ast.RangeStmt:
			recordObject(pass, n.Key, n.Pos(), assigns)
			recordObject(pass, n.Value, n.Pos(), assigns)
		case *ast.CallExpr:
			if lit, ok := ast.Unparen(n.Fun).(*ast.FuncLit); ok {
				collectCalls(pass, lit.Body, writes)
				return false
			}
			if obj, ok := writeObject(pass, n); ok {
				writes[obj] = append(writes[obj], n.Pos())
			}
		}
		return true
	})
}

// collectCalls records the writes in node, which is evaluated immediately.
func collectCalls(pass *analysis.Pass, node ast.Node, writes map[types.Object][]token.Pos) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit, *ast.GoStmt:
			return false
		case *ast.CallExpr:
			if obj, ok := writeObject(pass, n); ok {
				writes[obj] = append(writes[obj], n.Pos())
			}
		}
		return true
	})
}

// recordObject records pos for the object expr names.
func recordObject(pass *analysis.Pass, expr ast.Expr, pos token.Pos, out map[types.Object][]token.Pos) {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return
	}
	if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
		out[obj] = append(out[obj], pos)
	}
}

// writeObject returns the object that call writes through, if any.
func writeObject(pass *analysis.Pass, call *ast.CallExpr) (types.Object, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok {
		return nil, false
	}
	target, ok := writeTarget(pass, call, sel, fn)
	if !ok {
		return nil, false
	}
	id, ok := ast.Unparen(target).(*ast.Ident)
	if !ok {
		return nil, false
	}
	obj := pass.TypesInfo.ObjectOf(id)
	return obj, obj != nil
}

// writeMethod reports whether fn has the signature of a write method.
func writeMethod(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}
	switch fn.Name() {
	case "Write":
		return writerSignature(sig)
	case "WriteString":
		return stringSignature(sig)
	case "WriteAt":
		return atSignature(sig)
	}
	return false
}

// stringSignature reports whether sig is WriteString(string) (int, error).
func stringSignature(sig *types.Signature) bool {
	return sig.Params().Len() == 1 &&
		types.Identical(sig.Params().At(0).Type(), types.Typ[types.String]) &&
		intErrorResults(sig)
}

// atSignature reports whether sig is WriteAt([]byte, int64) (int, error).
func atSignature(sig *types.Signature) bool {
	if sig.Params().Len() != 2 || !intErrorResults(sig) {
		return false
	}
	bytes := types.NewSlice(types.Typ[types.Uint8])
	return types.Identical(sig.Params().At(0).Type(), bytes) &&
		types.Identical(sig.Params().At(1).Type(), types.Typ[types.Int64])
}

// writerSignature reports whether sig is Write([]byte) (int, error).
func writerSignature(sig *types.Signature) bool {
	if sig.Params().Len() != 1 || !intErrorResults(sig) {
		return false
	}
	bytes := types.NewSlice(types.Typ[types.Uint8])
	return types.Identical(sig.Params().At(0).Type(), bytes)
}

// intErrorResults reports whether sig returns (int, error).
func intErrorResults(sig *types.Signature) bool {
	return sig.Results().Len() == 2 &&
		types.Identical(sig.Results().At(0).Type(), types.Typ[types.Int]) &&
		types.Identical(
			sig.Results().At(1).Type(),
			types.Universe.Lookup("error").Type(),
		)
}

func writeTarget(pass *analysis.Pass, call *ast.CallExpr, sel *ast.SelectorExpr, fn *types.Func) (ast.Expr, bool) {
	if _, ok := methodSelection(pass, sel); ok && writeMethod(fn) {
		return methodReceiver(pass, call, sel), true
	}
	if len(call.Args) == 0 || fn.Pkg() == nil {
		return nil, false
	}
	switch fn.Pkg().Path() {
	case "fmt":
		switch fn.Name() {
		case "Fprint", "Fprintf", "Fprintln":
			return call.Args[0], true
		}
	case "io":
		switch fn.Name() {
		case "WriteString", "Copy", "CopyN", "CopyBuffer":
			return call.Args[0], true
		}
	}
	return nil, false
}

// methodSelection returns the selection for a method call. A field value and
// a package function have none.
func methodSelection(pass *analysis.Pass, sel *ast.SelectorExpr) (*types.Selection, bool) {
	s, ok := pass.TypesInfo.Selections[sel]
	return s, ok && s.Kind() != types.FieldVal
}

// methodReceiver returns the value a method call or method expression calls
// the method on.
func methodReceiver(pass *analysis.Pass, call *ast.CallExpr, sel *ast.SelectorExpr) ast.Expr {
	s, ok := pass.TypesInfo.Selections[sel]
	if ok && s.Kind() == types.MethodExpr && len(call.Args) > 0 {
		return call.Args[0]
	}
	return sel.X
}

// closesFile reports whether sel names the Close method of os.File.
func closesFile(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && fn.FullName() == "(*os.File).Close"
}

// flushes reports whether typ has a Flush method that returns an error.
func flushes(typ types.Type, addressable bool) bool {
	var (
		obj, _, _ = types.LookupFieldOrMethod(typ, addressable, nil, "Flush")
		fn, ok    = obj.(*types.Func)
	)
	if !ok {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && returnsError(sig)
}

// writes reports whether typ has an io.Writer Write method. The method set
// includes a promoted method and the method of an interface or a type
// parameter. A pointer method counts only when the receiver is addressable.
func writes(typ types.Type, addressable bool) bool {
	if typ == nil {
		return false
	}
	var (
		obj, _, _ = types.LookupFieldOrMethod(typ, addressable, nil, "Write")
		fn, ok    = obj.(*types.Func)
	)
	if !ok {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && writerSignature(sig)
}
