// Package boolset provides an analysis.Analyzer that finds sets
// dressed as map[T]bool.
//
// The rule reports one shape: a map created by make or a composite
// literal that stores only literal true and is read for membership.
// Any other value stored — false, a variable, a computed bool, by
// assignment or as a composite-literal element — makes the value state
// rather than membership, and the check does not report it. Neither
// does a read that uses the value, such as a comma-ok whose value is
// read or a range over the values. delete, clear and len mean the same
// thing for a set as for a bool map, so they are fine. A map that
// comes from elsewhere — a parameter, a struct field, an exported
// package variable, an aliased value — or is passed to a call is not
// examined, because not every store to it is in this package. An
// unexported package variable is examined: the package is analyzed as
// a whole, test files included, though a file that a build constraint
// excludes is not part of that whole, so a store there is not seen.
// Each variable is reported once, at its declaration.
package boolset

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "boolset",
	Doc:  "reports map[T]bool used as a set",
	Run:  run,
}

type candidate struct {
	def *ast.Ident
	bad bool
}

// checker is one package's candidates, keyed by object and in the order
// declared.
type checker struct {
	pass  *analysis.Pass
	byObj map[types.Object]*candidate
	cands []*candidate
}

func run(pass *analysis.Pass) (any, error) {
	c := &checker{pass: pass, byObj: make(map[types.Object]*candidate)}
	// A package variable is declared in one file and used in another, so
	// every declaration is collected before any file is walked. An
	// exported one is left alone: a caller can store false into it, and
	// that store is not here.
	for _, f := range pass.Files {
		c.declare(f)
	}
	for _, f := range pass.Files {
		c.checkFile(f)
	}
	c.report()
	return nil, nil
}

// declare records the package-level variables of f that a set can be.
func (c *checker) declare(f *ast.File) {
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			info := c.pass.TypesInfo
			for i, name := range vs.Names {
				obj, ok := info.Defs[name].(*types.Var)
				if !ok || !boolMap(obj.Type()) || obj.Exported() {
					continue
				}
				if i >= len(vs.Values) || !newSet(info, vs.Values[i]) {
					continue
				}
				cand := &candidate{def: name}
				c.byObj[obj] = cand
				c.cands = append(c.cands, cand)
			}
		}
	}
}

// report emits a diagnostic for each candidate that is only ever used as a
// set.
func (c *checker) report() {
	for _, cand := range c.cands {
		if cand.bad {
			continue
		}
		c.pass.Report(analysis.Diagnostic{
			Pos:     cand.def.Pos(),
			End:     cand.def.End(),
			Message: cand.def.Name + " is a set and can be map[T]struct{}",
		})
	}
}

func (c *checker) checkFile(f *ast.File) {
	var (
		stack []ast.Node
		info  = c.pass.TypesInfo
	)
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if obj, isDef := info.Defs[id].(*types.Var); isDef {
			if !boolMap(obj.Type()) || obj.Parent() == c.pass.Pkg.Scope() {
				return true
			}
			// Only a declaration whose initializer is here is a
			// candidate: a parameter, result, struct field, or range
			// variable arrives from elsewhere.
			cand := &candidate{def: id}
			switch parent := stack[len(stack)-2].(type) {
			case *ast.ValueSpec:
				for i, name := range parent.Names {
					if name != id {
						continue
					}
					cand.bad = i >= len(parent.Values) ||
						!newSet(info, parent.Values[i])
				}
			case *ast.AssignStmt:
				for i, lhs := range parent.Lhs {
					if lhs != id {
						continue
					}
					if i >= len(parent.Rhs) || !newSet(info, parent.Rhs[i]) {
						cand.bad = true
					}
				}
			default:
				return true
			}
			c.byObj[obj] = cand
			c.cands = append(c.cands, cand)
			return true
		}
		obj := info.Uses[id]
		cand := c.byObj[obj]
		if cand == nil {
			return true
		}
		classify(info, cand, stack)
		return true
	})
}

func boolMap(t types.Type) bool {
	m, ok := t.Underlying().(*types.Map)
	if !ok {
		return false
	}
	b, ok := m.Elem().Underlying().(*types.Basic)
	return ok && b.Kind() == types.Bool
}

// classify buckets one use of the candidate by its syntactic
// context, disqualifying the candidate where the shape proves
// meaningful state or an escape.
func classify(info *types.Info, c *candidate, stack []ast.Node) {
	switch parent := stack[len(stack)-2].(type) {
	case *ast.IndexExpr:
		classifyIndex(info, c, stack)
	case *ast.RangeStmt:
		if parent.Value != nil {
			c.bad = true // the bool values are read
		}
	case *ast.CallExpr:
		// Anything but a set-shaped builtin escapes into a call.
		fun, _ := parent.Fun.(*ast.Ident)
		switch b, _ := info.Uses[fun].(*types.Builtin); {
		case b == nil:
			c.bad = true
		case b.Name() == "len", b.Name() == "delete",
			b.Name() == "clear":
		default:
			c.bad = true
		}
	case *ast.AssignStmt:
		// On the left, a whole-map reassignment: the new source
		// must construct a set here. On the right, the map flows
		// into another variable.
		id := stack[len(stack)-1]
		for i, lhs := range parent.Lhs {
			if lhs != id {
				continue
			}
			if i >= len(parent.Rhs) || !newSet(info, parent.Rhs[i]) {
				c.bad = true
			}
			return
		}
		for _, rhs := range parent.Rhs {
			if rhs == id {
				c.bad = true
			}
		}
	default:
		c.bad = true
	}
}

// classifyIndex handles m[k] uses: a store of literal true is the
// set shape and any other store is meaningful state; a comma-ok
// read with a blank value is a membership test and one that
// consumes the value proves a real bool map. Any other read is a
// membership read and is fine.
func classifyIndex(info *types.Info, c *candidate, stack []ast.Node) {
	index := stack[len(stack)-2].(*ast.IndexExpr)
	switch container := stack[len(stack)-3].(type) {
	case *ast.ValueSpec:
		if len(container.Names) != 2 {
			return
		}
		if container.Names[0].Name != "_" {
			c.bad = true
		}
	case *ast.AssignStmt:
		assign := container
		for i, lhs := range assign.Lhs {
			if lhs != index {
				continue
			}
			if len(assign.Lhs) != 1 || !isTrue(info, assign.Rhs[i]) {
				c.bad = true
			}
			return
		}
		commaOK := len(assign.Lhs) == 2 && len(assign.Rhs) == 1 &&
			assign.Rhs[0] == index
		if commaOK {
			id, isID := assign.Lhs[0].(*ast.Ident)
			if !isID || id.Name != "_" {
				c.bad = true
			}
		}
	}
}

// isTrue reports whether e is the predeclared true.
func isTrue(info *types.Info, e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && info.Uses[id] == types.Universe.Lookup("true")
}

// newSet reports whether e constructs a set here: a call of the
// predeclared make, or a composite literal whose every element
// stores the predeclared true.
func newSet(info *types.Info, e ast.Expr) bool {
	switch node := e.(type) {
	case *ast.CallExpr:
		id, ok := node.Fun.(*ast.Ident)
		if !ok {
			return false
		}
		b, isBuiltin := info.Uses[id].(*types.Builtin)
		return isBuiltin && b.Name() == "make"
	case *ast.CompositeLit:
		for _, el := range node.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok || !isTrue(info, kv.Value) {
				return false
			}
		}
		return true
	}
	return false
}
