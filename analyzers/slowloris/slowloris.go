// Package slowloris provides an analysis.Analyzer that reports an http.Server
// literal with no timeout for reading headers.
//
// The Server documentation says of ReadHeaderTimeout that a negative value, or
// a zero while ReadTimeout is zero or negative, means no timeout. A client
// that sends its headers slowly holds the connection open, which is the
// slowloris attack.
//
// The README lists which assignments the rule follows and the shapes it cannot
// see.
//
// Report only: any duration would be a guess at the server's policy, so there
// is no fix.
//
// The rule is adapted from gosec's G112 (Apache License 2.0, Copyright 2016
// Hewlett Packard Enterprise Development LP). The check follows that project's
// rules/slowloris.go: a literal bound to the path that holds it, and a later
// assignment that gives it a timeout. It differs from that check in reading
// the two fields in net/http's order, and in following the whole path rather
// than the last field.
package slowloris

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "slowloris",
	Doc:  "reports an http.Server literal with no timeout for reading headers",
	Run:  run,
}

// The two fields net/http reads for the header deadline.
const (
	readHeaderTimeout = "ReadHeaderTimeout"
	readTimeout       = "ReadTimeout"
)

func run(pass *analysis.Pass) (any, error) {
	s := scan(pass)
	for _, lit := range s.literals {
		s.check(lit)
	}
	return nil, nil
}

// pkgScan holds one package's server literals, the path that holds each one,
// and every assignment to a path or to one of the two fields.
type pkgScan struct {
	pass     *analysis.Pass
	literals []*ast.CompositeLit
	holders  map[*ast.CompositeLit][]types.Object
	writes   []write
	binds    []bind
}

// write is an assignment to one of the two timeout fields of a path that holds
// a server.
type write struct {
	path   []types.Object
	header bool // the assignment is to ReadHeaderTimeout
	state  value
	pos    token.Pos
}

// value is what one of the two fields is written as.
type value int

const (
	none     value = iota // not written, or written as the constant zero
	negative              // written as a constant below zero
	positive              // above zero, or a value the rule cannot see
)

// deadline reports whether the pair leaves the header read with a deadline.
func deadline(header, read value) bool {
	// net/http reads ReadHeaderTimeout unless it is zero.
	if header != none {
		return header == positive
	}
	return read == positive
}

// bind is an assignment to a path, which changes what the path holds.
type bind struct {
	path []types.Object
	pos  token.Pos
}

// scan reads the package. A test file is left out: the only client a test
// serves is the test, and a write there does not configure the server the
// program serves.
func scan(pass *analysis.Pass) *pkgScan {
	s := &pkgScan{
		pass:    pass,
		holders: make(map[*ast.CompositeLit][]types.Object),
	}
	for _, file := range pass.Files {
		name := pass.Fset.File(file.Pos()).Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		s.file(file)
	}
	return s
}

// file reads one file of the package.
func (s *pkgScan) file(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				value := n.Rhs[min(i, len(n.Rhs)-1)]
				s.recordBind(lhs, value)
				s.recordWrite(lhs, stateOf(s.pass, value))
			}
		case *ast.IncDecStmt:
			// An increment or a decrement leaves a value the rule does not
			// compute, so it counts as a deadline.
			s.recordWrite(n.X, positive)
		case *ast.ValueSpec:
			for i, name := range n.Names {
				var value ast.Expr
				if len(n.Values) > 0 {
					value = n.Values[min(i, len(n.Values)-1)]
				}
				s.recordBind(name, value)
			}
		case *ast.CompositeLit:
			if isServer(s.pass, n) {
				s.literals = append(s.literals, n)
			}
		}
		return true
	})
}

// recordBind records the path an assignment writes, which ends what the path
// held before. It also records the server literals the value holds.
func (s *pkgScan) recordBind(target, value ast.Expr) {
	path, ok := s.path(target)
	if !ok {
		return
	}
	s.binds = append(s.binds, bind{path: path, pos: target.Pos()})
	if value != nil {
		s.hold(value, path)
	}
}

// hold records the server literals value holds at path, and the ones a field
// of it holds.
func (s *pkgScan) hold(value ast.Expr, path []types.Object) {
	lit := literal(value)
	if lit == nil {
		return
	}
	if isServer(s.pass, lit) {
		s.holders[lit] = path
		return
	}
	// Only a struct's keys name fields; a map's keys are values.
	t := s.pass.TypesInfo.TypeOf(lit)
	if t == nil {
		return
	}
	if _, ok := t.Underlying().(*types.Struct); !ok {
		return
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		field := s.pass.TypesInfo.Uses[key]
		if field == nil {
			continue
		}
		s.hold(kv.Value, append(slices.Clone(path), field))
	}
}

// recordWrite records a place where a path is given one of the two fields.
func (s *pkgScan) recordWrite(target ast.Expr, state value) {
	sel, ok := ast.Unparen(target).(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name
	if name != readHeaderTimeout && name != readTimeout {
		return
	}
	if !isServer(s.pass, sel.X) {
		return
	}
	path, ok := s.path(sel.X)
	if !ok {
		return
	}
	s.writes = append(s.writes, write{
		path:   path,
		header: name == readHeaderTimeout,
		state:  state,
		pos:    sel.Pos(),
	})
}

// check reports lit when the fields leave the header read with no deadline.
func (s *pkgScan) check(lit *ast.CompositeLit) {
	var (
		end          = lit.End()
		from         = s.pass.Fset.Position(end)
		path         = s.holders[lit]
		nextBind     = s.rebind(end, path)
		header, read = states(s.pass, lit)
	)
	for _, w := range s.writes {
		if !slices.Equal(w.path, path) {
			continue
		}
		if at := s.pass.Fset.Position(w.pos); at.Filename != from.Filename {
			// Another file names no order against this one, so a write
			// there can stand the report down and not bring it back.
			if w.state != positive {
				continue
			}
		} else if w.pos <= end {
			continue
		} else if nextBind != token.NoPos && w.pos > nextBind {
			continue
		}
		if w.header {
			header = w.state
		} else {
			read = w.state
		}
	}
	if deadline(header, read) {
		return
	}
	s.pass.Report(analysis.Diagnostic{
		Pos:     lit.Pos(),
		End:     lit.End(),
		Message: "no header read timeout; a client can hold the connection open",
	})
}

// rebind returns the position at which path is next written after end, or
// token.NoPos when it is not written again. A write of a path that holds this
// one ends it too, since the fields below that path change with it.
func (s *pkgScan) rebind(end token.Pos, path []types.Object) (next token.Pos) {
	for _, b := range s.binds {
		if len(b.path) > len(path) {
			continue
		}
		if !slices.Equal(b.path, path[:len(b.path)]) {
			continue
		}
		if b.pos <= end {
			continue
		}
		if next == token.NoPos || b.pos < next {
			next = b.pos
		}
	}
	return next
}

// path returns the variable or field that expr names, from the variable
// outward.
func (s *pkgScan) path(expr ast.Expr) ([]types.Object, bool) {
	switch x := ast.Unparen(expr).(type) {
	case *ast.Ident:
		obj := s.pass.TypesInfo.ObjectOf(x)
		if obj == nil {
			return nil, false
		}
		return []types.Object{obj}, true
	case *ast.SelectorExpr:
		outer, ok := s.path(x.X)
		if !ok {
			return nil, false
		}
		field := s.pass.TypesInfo.ObjectOf(x.Sel)
		if field == nil {
			return nil, false
		}
		return append(outer, field), true
	case *ast.StarExpr:
		return s.path(x.X)
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			return s.path(x.X)
		}
	}
	return nil, false
}

// stateOf reports what expr is written as. A value the rule cannot see may be
// above zero, so it counts as a deadline.
func stateOf(pass *analysis.Pass, expr ast.Expr) value {
	if expr == nil {
		return none
	}
	tv, ok := pass.TypesInfo.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return positive
	}
	switch constant.Sign(tv.Value) {
	case -1:
		return negative
	case 1:
		return positive
	}
	return none
}

// states returns what lit writes to the two timeout fields.
func states(pass *analysis.Pass, lit *ast.CompositeLit) (header, read value) {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case readHeaderTimeout:
			header = stateOf(pass, kv.Value)
		case readTimeout:
			read = stateOf(pass, kv.Value)
		}
	}
	return header, read
}

// literal returns the composite literal expr is written as, looking through
// parentheses and an address-of.
func literal(expr ast.Expr) *ast.CompositeLit {
	expr = ast.Unparen(expr)
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = ast.Unparen(unary.X)
	}
	lit, _ := expr.(*ast.CompositeLit)
	return lit
}

// isServer reports whether expr has the type net/http.Server, or a pointer to
// one.
func isServer(pass *analysis.Pass, expr ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "net/http" &&
		obj.Name() == "Server"
}
