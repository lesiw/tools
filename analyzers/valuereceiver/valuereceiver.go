// Package valuereceiver provides an analysis.Analyzer that reports a dead
// write to a field of a value receiver.
//
// The Go review comments state the rule as "If the method needs to mutate the
// receiver, the receiver must be a pointer." A value receiver works on a copy,
// so a write to its field is dead when nothing reads the field after it, apart
// from the reads that the writing statement itself makes, which a loop turns
// into reads of the next pass. The README states which reads save a write, and
// which methods are skipped.
//
// Report only: the repair an author wants is a pointer receiver, which changes
// behavior by sending the write to the caller's value, and it moves the method
// out of the type's method set, so a caller holding the value in an interface
// stops compiling.
//
// The idea comes from revive's modifies-value-receiver.
package valuereceiver

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "valuereceiver",
	Doc:  "reports a dead write to a field of a value receiver",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			if recv := valueReceiver(pass, fn); recv != nil {
				m := &method{pass: pass, recv: recv, body: fn.Body}
				m.check()
			}
		}
	}
	return nil, nil
}

// valueReceiver returns the receiver a method with a value receiver binds, or
// nil when the receiver is a pointer or is not named.
func valueReceiver(pass *analysis.Pass, fn *ast.FuncDecl) *types.Var {
	field := fn.Recv.List[0]
	if len(field.Names) == 0 {
		return nil
	}
	recv, ok := pass.TypesInfo.Defs[field.Names[0]].(*types.Var)
	if !ok {
		return nil
	}
	// A pointer receiver refers to the caller's value.
	if _, ok := recv.Type().(*types.Pointer); ok {
		return nil
	}
	return recv
}

// method is one method body, with the targets and writes found in it.
type method struct {
	pass    *analysis.Pass
	recv    *types.Var
	body    *ast.BlockStmt
	targets []ast.Expr // assignment targets that are chains of the receiver
	writes  []write    // the writes among them
}

// write is one assignment to a field of the receiver's copy.
type write struct {
	target ast.Expr       // the expression assigned to
	path   []types.Object // the fields it passes through, outward
	stmt   ast.Stmt       // the statement doing the writing, nil for a range
}

// name returns the fields the write passes through, joined by dots.
func (w write) name() string {
	names := make([]string, len(w.path))
	for i, f := range w.path {
		names[i] = f.Name()
	}
	return strings.Join(names, ".")
}

// check reports each write to a field of the receiver's copy that nothing in
// the body reads.
func (m *method) check() {
	m.scan()
	if m.unordered() {
		return
	}
	for _, w := range m.writes {
		reads := m.reads(m.body, w.stmt)
		if after(reads, w.target.End(), w.path) || m.delayed(w) {
			continue
		}
		m.pass.Report(analysis.Diagnostic{
			Pos: w.target.Pos(),
			End: w.target.End(),
			Message: "dead write to field " + w.name() +
				" of a value receiver",
		})
	}
}

// scan records the assignment targets that are chains of the receiver, and the
// writes to fields of the receiver's copy among them.
func (m *method) scan() {
	ast.Inspect(m.body, func(n ast.Node) bool {
		var (
			stmt    ast.Stmt
			targets []ast.Expr
		)
		switch n := n.(type) {
		case *ast.AssignStmt:
			stmt, targets = n, n.Lhs
		case *ast.IncDecStmt:
			stmt, targets = n, []ast.Expr{n.X}
		case *ast.RangeStmt:
			// A range clause assigns before each pass of the body, so no
			// read inside it is part of the write.
			if n.Key != nil {
				targets = append(targets, n.Key)
			}
			if n.Value != nil {
				targets = append(targets, n.Value)
			}
		default:
			return true
		}
		for _, target := range targets {
			fields, ok := m.path(target)
			if !ok {
				if base := m.indexed(target); base != nil {
					m.targets = append(m.targets, base)
				}
				// Any other target reads its operands, the receiver
				// among them.
				continue
			}
			m.targets = append(m.targets, target)
			if len(fields) > 0 {
				m.writes = append(m.writes, write{target, fields, stmt})
			}
		}
		return true
	})
}

// path returns the fields of the receiver's copy that expr passes through, in
// order outward from the receiver, and whether expr is such a chain at all. A
// promoted field is named by the fields it is embedded in, so its spelling
// does not change the path. A chain behind a pointer reads the caller's data.
func (m *method) path(expr ast.Expr) ([]types.Object, bool) {
	switch x := ast.Unparen(expr).(type) {
	case *ast.Ident:
		if m.pass.TypesInfo.ObjectOf(x) == m.recv {
			return nil, true
		}
	case *ast.SelectorExpr:
		selection, ok := m.pass.TypesInfo.Selections[x]
		if !ok || selection.Kind() != types.FieldVal || selection.Indirect() {
			return nil, false
		}
		fields, ok := m.path(x.X)
		if !ok {
			return nil, false
		}
		return append(fields, m.chain(x.X, selection.Index())...), true
	}
	return nil, false
}

// chain returns the fields of expr's type that index names, which are the
// fields a promoted selector passes through.
func (m *method) chain(expr ast.Expr, index []int) (path []types.Object) {
	t := m.pass.TypesInfo.TypeOf(expr)
	for _, i := range index {
		f := t.Underlying().(*types.Struct).Field(i)
		path = append(path, f)
		t = f.Type()
	}
	return path
}

// place reports whether expr names memory inside the receiver's copy: a field
// of it, or an element of an array it holds. A pointer the copy holds points
// elsewhere.
func (m *method) place(expr ast.Expr) bool {
	if _, ok := m.path(expr); ok {
		return true
	}
	switch x := ast.Unparen(expr).(type) {
	case *ast.SelectorExpr:
		selection, ok := m.pass.TypesInfo.Selections[x]
		return ok && selection.Kind() == types.FieldVal &&
			!selection.Indirect() && m.place(x.X)
	case *ast.IndexExpr:
		return m.array(x.X)
	}
	return false
}

// array reports whether expr is an array of the receiver's copy, whose
// elements live in the copy.
func (m *method) array(expr ast.Expr) bool {
	return m.place(expr) && astexpr.IsArray(m.pass.TypesInfo.TypeOf(expr))
}

// indexed returns the array of the copy that expr writes into, when part of
// expr indexes one.
func (m *method) indexed(expr ast.Expr) ast.Expr {
	var found ast.Expr
	ast.Inspect(expr, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		index, ok := n.(*ast.IndexExpr)
		if !ok || !m.array(index.X) {
			return true
		}
		// The span is the chain the write is rooted in, so an index computed
		// from a field of the copy is still a read.
		for {
			inner, ok := ast.Unparen(index.X).(*ast.IndexExpr)
			if !ok {
				break
			}
			index = inner
		}
		found = index.X
		return false
	})
	return found
}

// assigned reports whether pos lies inside an assignment target.
func (m *method) assigned(pos token.Pos) bool {
	for _, target := range m.targets {
		if holds(target, pos) {
			return true
		}
	}
	return false
}

// unordered reports whether the body hands out the copy's address, or contains
// a goto, either of which can move a read across a write.
func (m *method) unordered() bool {
	var found bool
	ast.Inspect(m.body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				found = m.place(n.X)
			}
		case *ast.SliceExpr:
			// A slice of an array is a pointer into the copy.
			found = m.array(n.X)
		case *ast.SelectorExpr:
			// A pointer-receiver method takes the copy's address.
			selection, ok := m.pass.TypesInfo.Selections[n]
			if !ok || selection.Kind() != types.MethodVal {
				return true
			}
			sig := selection.Obj().Type().(*types.Signature)
			if _, ptr := sig.Recv().Type().(*types.Pointer); ptr {
				found = m.place(n.X)
			}
		case *ast.BranchStmt:
			found = n.Tok == token.GOTO
		}
		return !found
	})
	return found
}

// fieldRead is one read of a chain of the receiver's fields, or of the whole
// copy when path is nil.
type fieldRead struct {
	path []types.Object
	pos  token.Pos
}

// reads returns the chains node reads of the receiver's copy, with each read's
// position. A read inside skip belongs to the write and is not counted.
func (m *method) reads(node ast.Node, skip ast.Stmt) (reads []fieldRead) {
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if m.readsIdent(id) && !holds(skip, id.Pos()) {
				reads = append(reads, fieldRead{nil, id.End()})
			}
			return true
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || holds(skip, sel.Pos()) {
			return true
		}
		// An increment or a compound assignment reads the field it writes, so
		// the target counts as a read. The receiver identifier does not: it
		// names the copy, not the field.
		if m.assigned(sel.Pos()) && !m.selfRead(sel.Pos()) {
			return true
		}
		selection, ok := m.pass.TypesInfo.Selections[sel]
		if !ok {
			return true
		}
		fields, ok := m.path(sel.X)
		if !ok {
			return true
		}
		// A chain that stops at a pointer the copy holds reads the field that
		// pointer sits in, which is the path so far. One that does not adds
		// the fields the selector passes through. A call reads a copy of what
		// it is called on, so it can read every field of that.
		if selection.Kind() == types.FieldVal && !selection.Indirect() {
			fields = append(fields, m.chain(sel.X, selection.Index())...)
		}
		reads = append(reads, fieldRead{fields, sel.End()})
		return false
	})
	return reads
}

// selfRead reports whether pos is a target that its own statement reads.
func (m *method) selfRead(pos token.Pos) bool {
	for _, w := range m.writes {
		if holds(w.target, pos) && astexpr.SelfReads(w.stmt) {
			return true
		}
	}
	return false
}

// readsIdent reports whether id reads the receiver rather than being the base
// of an assignment target.
func (m *method) readsIdent(id *ast.Ident) bool {
	return m.pass.TypesInfo.Uses[id] == m.recv && !m.assigned(id.Pos())
}

// after reports whether one of reads is at a position beyond pos and reaches
// the fields in path. A pos of token.NoPos asks whether any of them does.
func after(reads []fieldRead, pos token.Pos, path []types.Object) bool {
	for _, r := range reads {
		if r.pos > pos && overlaps(r.path, path) {
			return true
		}
	}
	return false
}

// overlaps reports whether one field chain is a prefix of the other, so the
// read and the write reach the same value, or a value that contains it.
func overlaps(a, b []types.Object) bool {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// delayed reports whether a loop or func literal reads the written fields. A
// loop that contains the write reads the field again on its next pass, in the
// writing statement or after it, and a func literal anywhere in the body
// counts, because the tool cannot tell when it runs. A literal's body is a
// function body, so the reads its writing statement makes are its own.
func (m *method) delayed(w write) bool {
	var found bool
	ast.Inspect(m.body, func(n ast.Node) bool {
		if found {
			return false
		}
		var (
			governed = true
			skip     ast.Stmt
		)
		switch n := n.(type) {
		case *ast.FuncLit:
			skip = w.stmt
		case *ast.ForStmt, *ast.RangeStmt:
			governed = holds(n, w.target.Pos())
		default:
			governed = false
		}
		if governed {
			found = after(m.reads(n, skip), token.NoPos, w.path)
		}
		return !found
	})
	return found
}

// holds reports whether pos lies within node. A nil node holds nothing.
func holds(node ast.Node, pos token.Pos) bool {
	return node != nil && node.Pos() <= pos && pos < node.End()
}
