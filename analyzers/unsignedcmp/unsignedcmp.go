// Package unsignedcmp provides an analysis.Analyzer that reports a comparison
// of an unsigned value against a limit of its type: the least value the type
// holds, or the greatest.
//
// The rule is adapted from staticcheck's SA4003, which also reports the bounds
// of a signed type. The README lists the bounds, the fixes, and the
// exemptions.
//
// A fix deletes the branch that cannot run, writes the body where the
// condition always holds, or writes true or false in place of the comparison.
package unsignedcmp

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "unsignedcmp",
	Doc:  "reports a comparison of an unsigned value against a limit of its type",
	Run:  run,
}

// checker holds one file, the comparisons the rule reports in it, and the fix
// each one offers.
type checker struct {
	pass     *analysis.Pass
	file     *ast.File
	src      []byte
	parents  map[ast.Node]ast.Node
	reads    map[types.Object]int
	writes   map[*ast.Ident]struct{}
	siblings map[types.Object][]types.Object
	findings []finding
}

// finding is a comparison whose result is fixed, and the fix it offers.
type finding struct {
	bin     *ast.BinaryExpr
	operand ast.Expr
	message string
	result  bool

	fix   analysis.SuggestedFix
	nodes []ast.Node
}

func run(pass *analysis.Pass) (any, error) {
	var (
		reads, writes = counted(pass)
		siblings      = switchVars(pass)
		checkers      = make([]*checker, 0, len(pass.Files))
	)
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		c := &checker{
			pass:     pass,
			file:     file,
			src:      src,
			parents:  astexpr.Parents(file),
			reads:    reads,
			writes:   writes,
			siblings: siblings,
		}
		c.findings = c.collect()
		for i := range c.findings {
			c.fix(&c.findings[i])
		}
		checkers = append(checkers, c)
	}
	// Every fix in this pass is applied at once, so a read counts as gone
	// when the fixes that stay delete it.
	removed := make(map[types.Object]int)
	for _, c := range checkers {
		c.keep(removed)
		c.report()
	}
	return nil, nil
}

// collect returns the comparisons in the file that the rule reports.
func (c *checker) collect() []finding {
	var found []finding
	ast.Inspect(c.file, func(n ast.Node) bool {
		bin, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		if f, ok := findingFor(c.pass, c.file, bin); ok {
			found = append(found, f)
		}
		return true
	})
	return found
}

// keep takes the fix away from a finding whose edit cannot be applied, and
// counts the reads that the fixes staying behind will delete.
func (c *checker) keep(removed map[types.Object]int) {
	var taken []analysis.TextEdit
	for i := range c.findings {
		f := &c.findings[i]
		if len(f.nodes) == 0 {
			continue
		}
		touch := c.touched(f.nodes)
		switch {
		case c.commented(f.fix.TextEdits...),
			!c.survives(f, touch, removed),
			clashes(f.fix.TextEdits, taken):
			f.fix, f.nodes = analysis.SuggestedFix{}, nil
			continue
		}
		for obj, n := range touch {
			removed[obj] += n
		}
		taken = append(taken, f.fix.TextEdits...)
	}
}

// survives reports whether every variable the fix touches keeps a read once
// the reads already counted as gone are gone.
func (c *checker) survives(f *finding, touch, removed map[types.Object]int) bool {
	for obj, n := range touch {
		if !c.keeps(obj, removed, n, f.nodes[0]) {
			return false
		}
	}
	return true
}

// clashes reports whether an edit covers text that an earlier one does.
func clashes(edits, taken []analysis.TextEdit) bool {
	for _, edit := range edits {
		for _, other := range taken {
			if edit.Pos < other.End && other.Pos < edit.End {
				return true
			}
		}
	}
	return false
}

// commented reports whether a comment sits inside an edit, or is left on a
// line that an edit deletes.
func (c *checker) commented(edits ...analysis.TextEdit) bool {
	for _, edit := range edits {
		var (
			file = c.pass.Fset.File(edit.Pos)
			lo   = file.Offset(edit.Pos)
			hi   = file.Offset(edit.End)
		)
		if len(edit.NewText) == 0 {
			lo = source.Start(c.src, lo)
			if hi > 0 && c.src[hi-1] != '\n' {
				hi = source.End(c.src, hi)
				if c.src[hi-1] == '\n' {
					hi--
				}
			}
		}
		for _, group := range c.file.Comments {
			start := file.Offset(group.Pos())
			if lo <= start && start < hi {
				return true
			}
		}
	}
	return false
}

// report sends the file's diagnostics, with a fix for each finding that has
// one left.
func (c *checker) report() {
	for _, f := range c.findings {
		diagnostic := analysis.Diagnostic{
			Pos:     f.bin.Pos(),
			End:     f.bin.End(),
			Message: f.message,
		}
		if len(f.nodes) > 0 {
			diagnostic.SuggestedFixes = []analysis.SuggestedFix{f.fix}
		}
		c.pass.Report(diagnostic)
	}
}

// counted returns the reads of every object in the package, and the
// identifiers a statement assigns to. An assignment is not a read: x = 1,
// x, y := 1, 2, and for x = range xs assign without reading, while x += 1 and
// x++ read as well.
func counted(pass *analysis.Pass) (reads map[types.Object]int, writes map[*ast.Ident]struct{}) {
	writes = make(map[*ast.Ident]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
					return true
				}
				target(writes, n.Lhs)
			case *ast.RangeStmt:
				target(writes, []ast.Expr{n.Key, n.Value})
			}
			return true
		})
	}
	reads = make(map[types.Object]int)
	for ident, obj := range pass.TypesInfo.Uses {
		if _, ok := writes[ident]; !ok {
			reads[obj]++
		}
	}
	return reads, writes
}

// target adds the identifiers an assignment targets to writes. Another
// expression on the left, such as a[i], reads what it names.
func target(writes map[*ast.Ident]struct{}, targets []ast.Expr) {
	for _, expr := range targets {
		if ident, ok := ast.Unparen(expr).(*ast.Ident); ok {
			writes[ident] = struct{}{}
		}
	}
}

// switchVars returns the variables a type switch declares, grouped by the
// switch that declares them: one clause reading the variable keeps it used
// for every clause.
func switchVars(pass *analysis.Pass) map[types.Object][]types.Object {
	groups := make(map[types.Object][]types.Object)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.TypeSwitchStmt:
				var members []types.Object
				for _, stmt := range n.Body.List {
					clause, ok := stmt.(*ast.CaseClause)
					if !ok {
						continue
					}
					if obj := pass.TypesInfo.Implicits[clause]; obj != nil {
						members = append(members, obj)
					}
				}
				for _, obj := range members {
					groups[obj] = members
				}
			}
			return true
		})
	}
	return groups
}

// findingFor returns the finding for one comparison, when the bound is a limit
// of the operand's type.
func findingFor(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr) (finding, bool) {
	var (
		value, bound = ast.Unparen(bin.X), ast.Unparen(bin.Y)
		op           = bin.Op
	)
	switch op {
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
	default:
		return finding{}, false
	}
	// So that each case below has the operand on the left.
	if constantOf(pass, value) != nil && constantOf(pass, bound) == nil {
		value, bound = bound, value
		op = reverse(op)
	}
	limit := constantOf(pass, bound)
	// The compiler folds a comparison of two constants, so there is nothing to
	// report.
	if limit == nil || constantOf(pass, value) != nil {
		return finding{}, false
	}
	t := pass.TypesInfo.TypeOf(value)
	basic, ok := unsigned(t)
	if !ok {
		return finding{}, false
	}
	name := types.TypeString(t, qualifier(pass, file))
	least := constant.MakeInt64(0)
	greatest, fixed := maxOf(basic)
	var result bool
	var message string
	switch {
	case constant.Compare(limit, token.EQL, least):
		// A bound can be zero on this target and nonzero on another: the
		// difference of two target-width numbers is one.
		if !portable(pass, bound) {
			return finding{}, false
		}
		switch op {
		case token.LSS:
			result = false
		case token.GEQ:
			result = true
		default:
			// n <= 0 and n > 0 compare n with zero, and the result varies.
			return finding{}, false
		}
		message = fmt.Sprintf("%s is never negative; this is always %t", name, result)
	case fixed && constant.Compare(limit, token.EQL, greatest):
		if !portable(pass, bound) {
			return finding{}, false
		}
		switch op {
		case token.GTR:
			result = false
		case token.LEQ:
			result = true
		default:
			// n >= max and n < max compare against the bound.
			return finding{}, false
		}
		message = fmt.Sprintf(
			"%s cannot exceed %s; this is always %t",
			name, greatest, result,
		)
	default:
		return finding{}, false
	}
	return finding{
		bin:     bin,
		operand: value,
		message: message,
		result:  result,
	}, true
}

// fix fills in the fix a finding offers, and clears it when no safe edit
// exists.
func (c *checker) fix(f *finding) {
	// Reading the operand is part of what the condition does, so a comparison
	// whose operand calls a function is reported without a fix.
	if astexpr.Effectful(c.pass, f.operand) || panics(c.pass, f.operand) {
		return
	}
	// A negation inverts the value, and the fix deletes the negation as well.
	var (
		node   = c.above(f.bin)
		result = f.result
		not    bool
	)
	for {
		unary, ok := c.parents[node].(*ast.UnaryExpr)
		if !ok || unary.Op != token.NOT {
			break
		}
		node, result, not = unary, !result, true
	}
	if guard, ok := c.parents[node].(*ast.IfStmt); ok && guard.Cond == node {
		c.branch(f, guard, result)
	} else if not || !c.term(f) {
		c.value(f, node, result)
	}
	// A labeled jump may be the only jump its label has.
	if jumps(f.nodes) {
		f.fix, f.nodes = analysis.SuggestedFix{}, nil
	}
}

// branch fills in the fix that deletes the part of a guard that cannot run,
// where the condition is always result.
func (c *checker) branch(f *finding, guard *ast.IfStmt, result bool) {
	// The init statement runs before the condition, and the fix would delete
	// what it does.
	if guard.Init != nil {
		return
	}
	var (
		body   = guard.Body
		hollow = len(body.List) == 0
	)
	switch {
	case guard.Else != nil && !result && !empty(guard.Else):
		f.fix = analysis.SuggestedFix{
			Message: "Keep the branch that runs",
			TextEdits: []analysis.TextEdit{c.removal(
				guard.Pos(),
				guard.Else.Pos(),
			)},
		}
		f.nodes = []ast.Node{guard.Cond, body}
	case !result, hollow:
		edit, ok := c.drop(guard)
		if !ok {
			return
		}
		f.fix = analysis.SuggestedFix{
			Message:   "Drop the branch",
			TextEdits: []analysis.TextEdit{edit},
		}
		f.nodes = []ast.Node{guard.Cond, body}
		if guard.Else != nil {
			f.nodes = append(f.nodes, guard.Else)
		}
	default:
		// The body always runs: the fix deletes the condition and the else
		// clause, and the braces stay because they scope what the body
		// declares. A body that may not fall through would leave the
		// statements after it unreachable.
		if c.leaves(guard) {
			return
		}
		f.fix = analysis.SuggestedFix{
			Message: "Drop the condition",
			TextEdits: []analysis.TextEdit{{
				Pos: guard.Pos(),
				End: body.Lbrace,
			}},
		}
		f.nodes = []ast.Node{guard.Cond}
		if guard.Else != nil {
			f.fix.TextEdits = append(f.fix.TextEdits, analysis.TextEdit{
				Pos: body.Rbrace + 1,
				End: guard.End(),
			})
			f.nodes = append(f.nodes, guard.Else)
		}
	}
}

func empty(node ast.Node) bool {
	block, ok := node.(*ast.BlockStmt)
	return ok && len(block.List) == 0
}

// leaves reports whether the body's last statement might not fall through, so
// that the statements after the if would be unreachable. Anything but a plain
// statement counts, and the comparison is then reported without a fix.
func (c *checker) leaves(guard *ast.IfStmt) bool {
	if c.last(guard) {
		return false
	}
	switch guard.Body.List[len(guard.Body.List)-1].(type) {
	case *ast.ExprStmt, *ast.AssignStmt, *ast.IncDecStmt, *ast.DeclStmt,
		*ast.SendStmt, *ast.GoStmt, *ast.DeferStmt, *ast.EmptyStmt:
		return false
	}
	return true
}

// last reports whether stmt is the last statement of the list holding it.
func (c *checker) last(stmt ast.Stmt) bool {
	var top ast.Node = stmt
	for node := c.parents[top]; node != nil; node = c.parents[node] {
		var list []ast.Stmt
		switch parent := node.(type) {
		case *ast.BlockStmt:
			list = parent.List
		case *ast.CaseClause:
			list = parent.Body
		case *ast.CommClause:
			list = parent.Body
		default:
			top = node
			continue
		}
		return len(list) > 0 && list[len(list)-1] == top
	}
	return true
}

// term fills in the fix that deletes a comparison from a conjunction or
// disjunction that has the same value without it, and reports whether it found
// one. The comparison has to be the side the other side replaces: a false
// disjunct or a true conjunct.
func (c *checker) term(f *finding) bool {
	var (
		node       = c.above(f.bin)
		parent, ok = c.parents[node].(*ast.BinaryExpr)
	)
	if !ok {
		return false
	}
	switch {
	case parent.Op == token.LOR && !f.result:
	case parent.Op == token.LAND && f.result:
	default:
		return false
	}
	other := parent.X
	if other == node {
		other = parent.Y
	}
	f.fix = analysis.SuggestedFix{
		Message: "Drop the comparison",
		TextEdits: []analysis.TextEdit{{
			Pos:     parent.Pos(),
			End:     parent.End(),
			NewText: c.text(other),
		}},
	}
	f.nodes = []ast.Node{f.bin}
	return true
}

// value fills in the fix that writes result in place of node.
func (c *checker) value(f *finding, node ast.Node, result bool) {
	if c.indirect(node) {
		return
	}
	f.fix = analysis.SuggestedFix{
		Message: "Write the value",
		TextEdits: []analysis.TextEdit{{
			Pos:     node.Pos(),
			End:     node.End(),
			NewText: []byte(strconv.FormatBool(result)),
		}},
	}
	f.nodes = []ast.Node{f.bin}
}

// indirect reports whether the comparison is part of a larger construct: the
// operand of a negation, a term of a conjunction or disjunction, a loop or
// switch header, a case condition, or a literal's key.
func (c *checker) indirect(from ast.Node) bool {
	for node := from; ; {
		switch parent := c.parents[node].(type) {
		case *ast.UnaryExpr:
			// A bool expression is only ever the operand of a negation.
			return true
		case *ast.BinaryExpr:
			return parent.Op == token.LAND || parent.Op == token.LOR
		case *ast.ForStmt:
			return parent.Cond == node
		case *ast.SwitchStmt:
			return parent.Tag == node
		case *ast.CaseClause:
			return true
		case *ast.KeyValueExpr:
			return parent.Key == node
		case ast.Expr:
			node = parent
		default:
			return false
		}
	}
}

func (c *checker) text(e ast.Expr) []byte {
	file := c.pass.Fset.File(e.Pos())
	return c.src[file.Offset(e.Pos()):file.Offset(e.End())]
}

// above returns node, or the outermost parentheses around it.
func (c *checker) above(node ast.Node) ast.Node {
	for {
		paren, ok := c.parents[node].(*ast.ParenExpr)
		if !ok {
			return node
		}
		node = paren
	}
}

// drop returns the edit that deletes an if statement. The edit removes the
// parent's else clause when the statement is that clause's only content, since
// an else keyword needs a statement after it.
func (c *checker) drop(guard *ast.IfStmt) (analysis.TextEdit, bool) {
	labeled, ok := c.parents[guard].(*ast.LabeledStmt)
	if ok && labeled.Stmt == guard {
		// The label would be left with no statement.
		return analysis.TextEdit{}, false
	}
	start, end := guard.Pos(), guard.End()
	parent, ok := c.parents[guard].(*ast.IfStmt)
	if ok && parent.Else == guard {
		start = parent.Body.Rbrace + 1
	}
	return c.removal(start, end), true
}

// removal returns the edit that deletes the range, along with the whole lines
// the range occupies when nothing else is on them, so that no blank line is
// left behind.
func (c *checker) removal(start, end token.Pos) analysis.TextEdit {
	var (
		file = c.pass.Fset.File(start)
		lo   = file.Offset(start)
		hi   = file.Offset(end)
	)
	if !source.Alone(c.src, lo, hi) {
		return analysis.TextEdit{Pos: start, End: end}
	}
	return analysis.TextEdit{
		Pos: file.Pos(source.Start(c.src, lo)),
		End: file.Pos(source.End(c.src, hi)),
	}
}

// touched returns the reads that deleting the nodes removes. A variable the
// nodes declare is deleted with them.
func (c *checker) touched(nodes []ast.Node) map[types.Object]int {
	var (
		given     = make(map[types.Object]struct{})
		mentioned = make(map[types.Object]int)
	)
	for _, node := range nodes {
		ast.Inspect(node, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if obj := c.pass.TypesInfo.Defs[ident]; obj != nil {
				given[obj] = struct{}{}
			}
			if _, ok := c.writes[ident]; ok {
				return true
			}
			if obj := c.pass.TypesInfo.Uses[ident]; obj != nil {
				mentioned[obj]++
			}
			return true
		})
	}
	for obj := range given {
		delete(mentioned, obj)
	}
	return mentioned
}

// keeps reports whether a read of obj survives the deletions already counted
// in removed, along with the n this finding removes. A package-level variable,
// a field, a variable of a signature, and one clause's variable of a type
// switch may go unread, and the compiler rejects only a local that nothing
// reads.
func (c *checker) keeps(obj types.Object, removed map[types.Object]int, n int, from ast.Node) bool {
	variable, ok := obj.(*types.Var)
	if !ok || variable.IsField() {
		return true
	}
	if variable.Parent() == c.pass.Pkg.Scope() {
		return true
	}
	if c.signed(from, variable) {
		return true
	}
	// A type switch declares the variable again in every clause, and one
	// clause reading it keeps it used.
	if members, ok := c.siblings[obj]; ok {
		for _, member := range members {
			gone := removed[member]
			if member == obj {
				gone += n
			}
			if c.reads[member] > gone {
				return true
			}
		}
		return false
	}
	return c.reads[obj] > removed[obj]+n
}

// signed reports whether obj is a parameter, a result, or a receiver of a
// function that contains from, which the compiler allows to go unread.
func (c *checker) signed(from ast.Node, obj types.Object) bool {
	for node := c.parents[from]; node != nil; node = c.parents[node] {
		var (
			sig  *ast.FuncType
			decl ast.Node
		)
		switch n := node.(type) {
		case *ast.FuncDecl:
			sig, decl = n.Type, n
		case *ast.FuncLit:
			sig, decl = n.Type, n
		default:
			continue
		}
		if decl.Pos() <= obj.Pos() && obj.Pos() < sig.End() {
			return true
		}
	}
	return false
}

// jumps reports whether the nodes hold a labeled jump, which may be the only
// jump its label has.
func jumps(nodes []ast.Node) bool {
	var found bool
	for _, node := range nodes {
		ast.Inspect(node, func(n ast.Node) bool {
			if branch, ok := n.(*ast.BranchStmt); ok && branch.Label != nil {
				found = true
			}
			return !found
		})
	}
	return found
}

// panics reports whether evaluating e can panic: a dereference, an index, a
// type assertion, a division or a remainder, or a field reached through a
// pointer. The fix deletes the evaluation, so the panic goes with it.
func panics(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.StarExpr, *ast.SliceExpr, *ast.TypeAssertExpr:
			found = true
		case *ast.IndexExpr:
			found = indexable(pass.TypesInfo.TypeOf(n.X))
		case *ast.BinaryExpr:
			found = n.Op == token.QUO || n.Op == token.REM
		case *ast.SelectorExpr:
			if sel, ok := pass.TypesInfo.Selections[n]; ok {
				_, pointer := sel.Recv().Underlying().(*types.Pointer)
				found = pointer
			}
		}
		return !found
	})
	return found
}

// indexable reports whether indexing t can panic. A map index yields a zero
// value instead.
func indexable(t types.Type) bool {
	if t == nil {
		return false
	}
	_, mapped := t.Underlying().(*types.Map)
	return !mapped
}

// unsigned returns the basic type under t when t holds an unsigned integer. A
// type parameter has no basic underlying type, so no comparison of one is
// reported.
func unsigned(t types.Type) (*types.Basic, bool) {
	if t == nil {
		return nil, false
	}
	basic, ok := t.Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsUnsigned == 0 {
		return nil, false
	}
	return basic, true
}

// maxOf returns the greatest value a basic unsigned type holds, and whether
// that value is the same on every target.
func maxOf(basic *types.Basic) (constant.Value, bool) {
	var bits uint
	switch basic.Kind() {
	case types.Uint8:
		bits = 8
	case types.Uint16:
		bits = 16
	case types.Uint32:
		bits = 32
	case types.Uint64:
		bits = 64
	default:
		return nil, false
	}
	one := constant.MakeInt64(1)
	greatest := constant.BinaryOp(
		constant.Shift(one, token.SHL, bits),
		token.SUB,
		one,
	)
	return greatest, true
}

// portable reports whether a bound holds the same number on every target.
func portable(pass *analysis.Pass, e ast.Expr) bool {
	same := true
	ast.Inspect(e, func(n ast.Node) bool {
		if !same {
			return false
		}
		switch n := n.(type) {
		case *ast.Ident:
			same = fixedIdent(pass, n)
		case *ast.SelectorExpr:
			same = fixedIdent(pass, n.Sel)
			return false // the qualifier is a package name
		case *ast.CallExpr:
			same = !wide(pass.TypesInfo.Types[n].Type)
		}
		return same
	})
	return same
}

// fixedIdent reports whether ident is a type name or one of the math package's
// maxima for a fixed-width type. A constant of another name can be declared
// with another value in a file that a different build compiles, so this target
// does not fix what it holds.
func fixedIdent(pass *analysis.Pass, ident *ast.Ident) bool {
	switch obj := pass.TypesInfo.Uses[ident].(type) {
	case *types.TypeName:
		return true
	case *types.Const:
		return fixedMax(obj)
	}
	return false
}

// fixedMax reports whether c is a math maximum for a fixed-width type.
func fixedMax(c *types.Const) bool {
	if c.Pkg() == nil || c.Pkg().Path() != "math" {
		return false
	}
	switch c.Name() {
	case "MaxUint8", "MaxUint16", "MaxUint32", "MaxUint64":
		return true
	}
	return false
}

// wide reports whether t is as wide as the target's pointer.
func wide(t types.Type) bool {
	if t == nil {
		return false
	}
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	kind := basic.Kind()
	return kind == types.Int || kind == types.Uint || kind == types.Uintptr
}

// constantOf returns the value of e. TypesInfo records a value only for an
// expression the compiler evaluates, so a read at run time gives nil.
func constantOf(pass *analysis.Pass, e ast.Expr) constant.Value {
	return pass.TypesInfo.Types[e].Value
}

// qualifier prints a type from another package by the name the file gives it.
func qualifier(pass *analysis.Pass, file *ast.File) types.Qualifier {
	names := make(map[*types.Package]string)
	for _, spec := range file.Imports {
		var obj types.Object
		if spec.Name != nil {
			obj = pass.TypesInfo.Defs[spec.Name]
		} else {
			obj = pass.TypesInfo.Implicits[spec]
		}
		name, ok := obj.(*types.PkgName)
		if !ok {
			continue
		}
		local := name.Name()
		if local == "." {
			local = ""
		}
		names[name.Imported()] = local
	}
	return func(p *types.Package) string {
		if p == pass.Pkg {
			return ""
		}
		if name, ok := names[p]; ok {
			return name
		}
		return p.Name()
	}
}

// reverse returns the operator with its operands swapped.
func reverse(op token.Token) token.Token {
	switch op {
	case token.LSS:
		return token.GTR
	case token.LEQ:
		return token.GEQ
	case token.GTR:
		return token.LSS
	case token.GEQ:
		return token.LEQ
	default:
		return op
	}
}
