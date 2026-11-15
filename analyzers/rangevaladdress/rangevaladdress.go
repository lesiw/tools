// Package rangevaladdress provides an analysis.Analyzer that reports an
// address taken from a range loop's variable and kept past the iteration.
//
// Two cases have a shared loop variable: a file that declares a Go version
// before 1.22, where the loop has one variable for every iteration, and a
// range clause that assigns its variables with =, which shares them at every
// version. The spec gives the variables one instance per iteration from Go
// 1.22, and only when the clause declares them. An address taken from the
// variable is therefore the same address in every iteration, and a pointer
// kept past the iteration reads the value the loop wrote last.
//
// A report offers an edit that declares a copy of the variable where the copy
// cannot change what the body means.
//
// The rule is adapted from revive's range-val-address, which reads a range
// clause's value variable, does not check the key, and skips a package at Go
// 1.22 or later.
//
// The README catalogs every check, exemption, and fix.
package rangevaladdress

import (
	"go/ast"
	"go/token"
	"go/types"
	"go/version"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "rangevaladdress",
	Doc:  "reports an address taken from a range loop's variable and kept past the iteration",
	Run:  run,
}

// loop holds one range statement under analysis.
type loop struct {
	pass    *analysis.Pass
	file    *ast.File
	stmt    *ast.RangeStmt
	vars    []*types.Var
	carried map[types.Object][]address
	found   []address
}

// address is one address taken of a loop variable.
type address struct {
	expr *ast.UnaryExpr
	v    *types.Var
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// An empty version means the type checker enforced none, so the
		// language is the toolchain's, which gives each iteration its own
		// variable.
		var (
			v            = pass.TypesInfo.FileVersions[file]
			perIteration = v == "" || version.Compare(v, "go1.22") >= 0
		)
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.RangeStmt)
			if !ok {
				return true
			}
			// A loop that declares its variables gives each iteration its
			// own at Go 1.22, and a loop that assigns them shares them
			// always.
			if perIteration && stmt.Tok == token.DEFINE {
				return true
			}
			l := &loop{
				pass:    pass,
				file:    file,
				stmt:    stmt,
				vars:    loopvars(pass, stmt),
				carried: make(map[types.Object][]address),
			}
			l.report()
			return true
		})
	}
	return nil, nil
}

// loopvars returns the variables the range statement writes, which := declares
// and = names from the enclosing scope. One variable holds the value of every
// iteration of a loop that assigns with =, at every version.
func loopvars(pass *analysis.Pass, stmt *ast.RangeStmt) (out []*types.Var) {
	for _, e := range []ast.Expr{stmt.Key, stmt.Value} {
		id, ok := e.(*ast.Ident)
		if !ok {
			continue
		}
		if v, ok := pass.TypesInfo.ObjectOf(id).(*types.Var); ok {
			out = append(out, v)
		}
	}
	return out
}

// report reports each address of a loop variable that the body keeps past the
// iteration.
func (l *loop) report() {
	ast.Inspect(l.stmt.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			// The walk stops here because a literal is reported where it
			// reaches a target that outlives the iteration, in addresses.
			return false
		}
		if inner, ok := n.(*ast.RangeStmt); ok && l.writes(inner) {
			// A nested loop that writes the same variable reports the address
			// in its own body, where the copy belongs.
			return false
		}
		switch n := n.(type) {
		case *ast.AssignStmt:
			l.stores(n)
		case *ast.DeclStmt:
			l.declares(n)
		case *ast.IncDecStmt:
			l.incDec(n)
		case *ast.SendStmt:
			l.send(n)
		case *ast.GoStmt:
			l.calls(n, n.Call.Args)
		case *ast.DeferStmt:
			l.calls(n, n.Call.Args)
		}
		return true
	})
	l.emit()
}

// emit reports each address found once, with the edit that declares the
// copies.
func (l *loop) emit() {
	var (
		seen  = make(map[token.Pos]struct{}, len(l.found))
		vars  []*types.Var
		diags = make([]analysis.Diagnostic, 0, len(l.found))
	)
	for _, a := range l.found {
		if _, dup := seen[a.expr.Pos()]; dup {
			continue
		}
		seen[a.expr.Pos()] = struct{}{}
		diags = append(diags, l.diagnostic(a))
		if !slices.Contains(vars, a.v) {
			vars = append(vars, a.v)
		}
	}
	if len(diags) > 0 {
		// One edit opens the body, so only the first report carries it: a
		// copy on each would conflict.
		if fix, ok := l.copyFix(vars); ok {
			diags[0].SuggestedFixes = []analysis.SuggestedFix{fix}
		}
	}
	for _, d := range diags {
		l.pass.Report(d)
	}
}

// writes reports whether stmt writes a variable this loop writes.
func (l *loop) writes(stmt *ast.RangeStmt) bool {
	for _, v := range loopvars(l.pass, stmt) {
		if slices.Contains(l.vars, v) {
			return true
		}
	}
	return false
}

// stores reports the addresses of a loop variable that an assignment keeps
// past the iteration.
func (l *loop) stores(assign *ast.AssignStmt) {
	if l.stops(assign) {
		// Nothing writes the variable again: the value the address reads is
		// the one this iteration gave it.
		return
	}
	// A single right side, as in v, ok = m[k], is not one value per target, so
	// the two sides do not pair.
	if len(assign.Lhs) != len(assign.Rhs) {
		return
	}
	for i, lhs := range assign.Lhs {
		if l.outlives(lhs) {
			// An address written as the index of a map target is a key,
			// which the map stores the way a target does.
			if index, ok := ast.Unparen(lhs).(*ast.IndexExpr); ok {
				l.reportAddr(index.Index)
			}
			l.reportAddr(assign.Rhs[i])
			continue
		}
		// A target the body declares keeps the address for a later store.
		l.carry(lhs, assign.Rhs[i], false)
	}
}

// carry records the addresses target is given, for the stores that meet them
// later. A field, an element, or a key adds to what the variable already
// carries, where a variable that is declared or assigned outright takes the
// value it is given.
func (l *loop) carry(target, value ast.Expr, direct bool) {
	obj, _, ok := l.origin(target)
	if !ok || obj == nil {
		return
	}
	carried := l.addresses(value)
	if index, ok := ast.Unparen(target).(*ast.IndexExpr); ok {
		direct = false
		carried = append(carried, l.addresses(index.Index)...)
	}
	if _, ident := ast.Unparen(target).(*ast.Ident); !ident {
		direct = false
	}
	if direct {
		l.carried[obj] = carried
		return
	}
	l.carried[obj] = append(l.carried[obj], carried...)
}

// declares carries the addresses a var declaration gives its variables.
func (l *loop) declares(stmt *ast.DeclStmt) {
	decl, ok := stmt.Decl.(*ast.GenDecl)
	if !ok {
		return
	}
	for _, spec := range decl.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, id := range value.Names {
			if i >= len(value.Values) {
				break
			}
			l.carry(id, value.Values[i], true)
		}
	}
}

// incDec reports the address an increment or a decrement uses as a map key.
func (l *loop) incDec(stmt *ast.IncDecStmt) {
	if l.stops(stmt) {
		return
	}
	index, ok := ast.Unparen(stmt.X).(*ast.IndexExpr)
	if ok && l.outlives(index.X) {
		l.reportAddr(index.Index)
	}
}

// calls reports the addresses of a loop variable that a deferred or spawned
// call takes, since the call may read them after a later iteration writes the
// variable.
func (l *loop) calls(stmt ast.Stmt, args []ast.Expr) {
	if l.stops(stmt) {
		return
	}
	for _, arg := range args {
		l.reportAddr(arg)
	}
}

// send reports the addresses of a loop variable that a send hands to a reader,
// which reads them after the iteration ends.
func (l *loop) send(sent *ast.SendStmt) {
	if l.stops(sent) {
		return
	}
	if l.outlives(sent.Chan) {
		l.reportAddr(sent.Value)
		return
	}
	// A channel the body declares carries the address if the channel is
	// stored where it outlives the iteration.
	if obj, _, ok := l.origin(sent.Chan); ok && obj != nil {
		l.carried[obj] = append(l.carried[obj], l.addresses(sent.Value)...)
	}
}

// reportAddr collects the addresses of a loop variable that e carries.
func (l *loop) reportAddr(e ast.Expr) {
	l.found = append(l.found, l.addresses(e)...)
}

// diagnostic returns the report for one address.
func (*loop) diagnostic(a address) analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos: a.expr.Pos(),
		End: a.expr.End(),
		Message: "address of loop variable " + a.v.Name() +
			" is the same in every iteration",
	}
}

// assigns reports whether the body writes v.
func (l *loop) assigns(v *types.Var) bool {
	var found bool
	ast.Inspect(l.stmt.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				if id, ok := ast.Unparen(lhs).(*ast.Ident); ok {
					if l.pass.TypesInfo.ObjectOf(id) == v {
						found = true
					}
				}
			}
		case *ast.IncDecStmt:
			if id, ok := ast.Unparen(n.X).(*ast.Ident); ok {
				if l.pass.TypesInfo.ObjectOf(id) == v {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// literalReads reports whether a function literal in the body reads v. The
// copy would shadow the variable there, and the literal would read the copy.
func (l *loop) literalReads(v *types.Var) bool {
	var found bool
	ast.Inspect(l.stmt.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			found = found || l.mentions(n, v)
			return false
		}
		return !found
	})
	return found
}

// copyFix returns the fix that declares a copy of each variable as the body's
// first statement, so an address taken in the body is the copy's. It reports
// false where the copy would change what the body means, or where the first
// statement shares its line with the code before it.
func (l *loop) copyFix(vars []*types.Var) (analysis.SuggestedFix, bool) {
	var (
		none  analysis.SuggestedFix
		body  = l.stmt.Body
		scope = l.pass.TypesInfo.Scopes[body]
		names = make([]string, 0, len(vars))
	)
	for _, v := range vars {
		if scope != nil && scope.Lookup(v.Name()) != nil {
			// The body declares the name at its own level.
			return none, false
		}
		if l.literalReads(v) {
			return none, false
		}
		// A loop that assigns its variable shares it with the code after the
		// loop, which the body's writes would no longer reach.
		if l.stmt.Tok != token.DEFINE && l.assigns(v) {
			return none, false
		}
		names = append(names, v.Name())
	}
	first := body.List[0]
	tokFile := l.pass.Fset.File(body.Pos())
	src, err := l.pass.ReadFile(tokFile.Name())
	if err != nil {
		return none, false
	}
	line := tokFile.Offset(tokFile.LineStart(tokFile.Line(first.Pos())))
	at := tokFile.Offset(first.Pos())
	lead := string(src[line:at])
	if strings.TrimSpace(lead) != "" {
		// The first statement shares its line with code before it, so the
		// declaration has no line of its own.
		return none, false
	}
	return analysis.SuggestedFix{
		Message: "Copy " + strings.Join(names, ", ") + " into the body",
		TextEdits: []analysis.TextEdit{{
			Pos: tokFile.Pos(line),
			End: tokFile.Pos(line),
			NewText: []byte(lead +
				strings.Join(names, ", ") + " := " +
				strings.Join(names, ", ") + "\n",
			),
		}},
	}, true
}

// mentions reports whether n reads v.
func (l *loop) mentions(n ast.Node, v *types.Var) bool {
	var found bool
	ast.Inspect(n, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && l.pass.TypesInfo.ObjectOf(id) == v {
			found = true
		}
		return !found
	})
	return found
}

// addresses returns the addresses of a loop variable that e carries: one taken
// of it directly, one a variable the body assigned holds, one anywhere inside
// a composite literal, one inside an append or a conversion, and one inside a
// literal whose own address is taken.
func (l *loop) addresses(e ast.Expr) (out []address) {
	if a, ok := l.addressOf(e); ok {
		return []address{a}
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return l.carried[l.pass.TypesInfo.ObjectOf(e)]
	case *ast.CompositeLit:
		for _, elt := range e.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				// A map literal's key holds the address; a struct
				// literal's key is a field name.
				out = append(out, l.addresses(kv.Key)...)
				elt = kv.Value
			}
			out = append(out, l.addresses(elt)...)
		}
	case *ast.CallExpr:
		if !l.keeps(e) {
			return out
		}
		for _, arg := range e.Args {
			out = append(out, l.addresses(arg)...)
		}
	case *ast.FuncLit:
		// The literal is stored where it outlives the iteration, so the
		// addresses its body takes outlive it too.
		ast.Inspect(e.Body, func(n ast.Node) bool {
			if expr, ok := n.(ast.Expr); ok {
				if a, ok := l.addressOf(expr); ok {
					out = append(out, a)
				}
			}
			return true
		})
	case *ast.UnaryExpr:
		// A literal's address is new memory, but what it holds is not.
		if e.Op == token.AND {
			out = append(out, l.addresses(e.X)...)
		}
	}
	return out
}

// keeps reports whether a call's result still carries the addresses its
// arguments carry. The result of append does, and the result of a conversion
// is its operand; what any other call keeps cannot be seen from the call.
func (l *loop) keeps(call *ast.CallExpr) bool {
	fn, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if ok && l.pass.TypesInfo.Uses[fn] == types.Universe.Lookup("append") {
		return true
	}
	return len(call.Args) == 1 && l.pass.TypesInfo.Types[call.Fun].IsType()
}

// addressOf returns the address e takes of a loop variable, and reports false
// when e takes no such address.
func (l *loop) addressOf(e ast.Expr) (address, bool) {
	var (
		none      address
		unary, ok = ast.Unparen(e).(*ast.UnaryExpr)
	)
	if !ok || unary.Op != token.AND {
		return none, false
	}
	base := ast.Unparen(unary.X)
	for {
		inner := l.operand(base)
		if inner == nil {
			break
		}
		base = inner
	}
	id, ok := base.(*ast.Ident)
	if !ok {
		return none, false
	}
	v, ok := l.pass.TypesInfo.ObjectOf(id).(*types.Var)
	if !ok || !slices.Contains(l.vars, v) {
		return none, false
	}
	return address{expr: unary, v: v}, true
}

// operand returns the expression a selector or an array index reads its memory
// from, and nil when the memory is not the loop variable's own. A field of a
// struct or an element of an array held by the variable is part of it; one
// reached through a pointer or a slice belongs to whatever that refers to,
// which each iteration can place elsewhere.
func (l *loop) operand(e ast.Expr) ast.Expr {
	var inner ast.Expr
	switch e := e.(type) {
	case *ast.SelectorExpr:
		// A promoted field reached through an embedded pointer lives in
		// whatever that pointer points at.
		if sel, ok := l.pass.TypesInfo.Selections[e]; ok && sel.Indirect() {
			return nil
		}
		inner = e.X
	case *ast.IndexExpr:
		typ := l.pass.TypesInfo.TypeOf(e.X)
		if typ == nil {
			return nil
		}
		if _, ok := typ.Underlying().(*types.Array); !ok {
			return nil
		}
		inner = e.X
	default:
		return nil
	}
	if typ := l.pass.TypesInfo.TypeOf(inner); typ != nil {
		if _, through := typ.Underlying().(*types.Pointer); through {
			return nil
		}
	}
	return ast.Unparen(inner)
}

// outlives reports whether a store to target survives the iteration. A
// variable the body declares is local to the iteration, since its scope ends
// with the body, and so is a field of a struct value held by the body, since
// the value is a copy. Every other target lives at least as long as the loop.
// The body is one span, so a declaration inside it has a position inside it.
func (l *loop) outlives(target ast.Expr) bool {
	obj, copy, ok := l.origin(target)
	if !ok {
		// The store goes through a value the body did not declare.
		return true
	}
	if obj == nil {
		return false
	}
	if obj.Pos() >= l.stmt.Body.Pos() && obj.Pos() < l.stmt.Body.End() {
		return false
	}
	return !copy || obj.Parent() == l.pass.Pkg.Scope()
}

// origin returns the object the target's memory starts with, and whether the
// walk crossed a struct or an array held by value. It reports false for a
// target that names nothing, and true with no object for the blank identifier.
func (l *loop) origin(target ast.Expr) (_ types.Object, _, _ bool) {
	for {
		switch t := ast.Unparen(target).(type) {
		case *ast.Ident:
			typ := l.pass.TypesInfo.TypeOf(t)
			return l.pass.TypesInfo.ObjectOf(t), value(typ), true
		case *ast.SelectorExpr:
			target = t.X
		case *ast.IndexExpr:
			target = t.X
		case *ast.StarExpr:
			target = t.X
		default:
			return nil, false, false
		}
	}
}

// value reports whether t is a struct or an array, whose memory a copy holds.
func value(t types.Type) bool {
	if t == nil {
		return false
	}
	switch t.Underlying().(type) {
	case *types.Struct, *types.Array:
		return true
	}
	return false
}

// stops reports whether the loop ends after stmt: the next statement to run is
// a return, a break that leaves the loop, or a goto past it.
func (l *loop) stops(stmt ast.Stmt) bool {
	var (
		path, _ = astutil.PathEnclosingInterval(l.file, stmt.Pos(), stmt.End())
		child   ast.Stmt
	)
	for _, node := range path {
		if tail, ok := after(node, child); ok {
			if ends, settled := l.tailEnds(tail); settled {
				return ends
			}
		}
		if s, ok := node.(ast.Stmt); ok {
			child = s
		}
		// The walk has left the body: the iteration continues.
		if node == ast.Node(l.stmt) {
			return false
		}
	}
	return false
}

// after returns the statements that follow child in node. In a select, the
// clause's body runs after its send or receive.
func after(node ast.Node, child ast.Stmt) ([]ast.Stmt, bool) {
	var list []ast.Stmt
	switch n := node.(type) {
	case *ast.BlockStmt:
		list = n.List
	case *ast.CaseClause:
		list = n.Body
	case *ast.CommClause:
		if n.Comm == child {
			return n.Body, true
		}
		list = n.Body
	default:
		return nil, false
	}
	i := slices.Index(list, child)
	if i < 0 {
		return nil, false
	}
	return list[i+1:], true
}

// tailEnds reports whether the loop ends in tail before anything there runs
// the next iteration, and whether tail holds a statement that transfers
// control at all. A list that runs out has answered nothing, and the walk
// reads on around it.
func (l *loop) tailEnds(tail []ast.Stmt) (ends, settled bool) {
	for _, stmt := range tail {
		switch stmt.(type) {
		case *ast.EmptyStmt, *ast.ExprStmt, *ast.AssignStmt, *ast.SendStmt,
			*ast.IncDecStmt, *ast.DeclStmt, *ast.GoStmt, *ast.DeferStmt:
			continue
		}
		return l.ends(stmt), true
	}
	return false, false
}

// ends reports whether stmt ends the loop or the function it runs in. A goto
// leaves the loop for good only when its label is after it; a label on the
// loop or before it runs the loop again.
func (l *loop) ends(stmt ast.Stmt) bool {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		switch s.Tok {
		case token.BREAK:
			return l.leavesLoop(s)
		case token.GOTO:
			label := l.pass.TypesInfo.Uses[s.Label]
			return label != nil && label.Pos() > l.stmt.End()
		}
	}
	return false
}

// leavesLoop reports whether branch leaves the loop. A break leaves the
// innermost for, range, switch, or select around it, or the statement a label
// names.
func (l *loop) leavesLoop(branch *ast.BranchStmt) bool {
	if branch.Label != nil {
		// A label is written before the statement it names, so a label before
		// the loop names the loop or a statement around it; the break leaves
		// the loop either way.
		label := l.pass.TypesInfo.Uses[branch.Label]
		return label != nil && label.Pos() < l.stmt.Pos()
	}
	path, _ := astutil.PathEnclosingInterval(
		l.file, branch.Pos(),
		branch.End(),
	)
	for _, node := range path {
		if node == ast.Node(l.stmt) {
			// The break leaves the loop.
			return true
		}
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.SelectStmt:
			// The break leaves this, and the loop goes on.
			return false
		}
	}
	return false
}
