// Package exposedsyncmutex provides an analysis.Analyzer that reports a
// sync.Mutex or sync.RWMutex embedded in a type an importer can reach.
//
// An embedded field is named after its type, so the embed gives the struct a
// field named Mutex. For an embedded sync.Mutex the pointer to the type gains
// Lock and Unlock, and for an embedded *sync.Mutex the type itself gains them,
// so a value of the type satisfies sync.Locker. The specification's "Struct
// types" section states both rules. A caller can unlock the mutex a method
// holds, so the invariant the lock exists for no longer holds, and a caller
// that holds the lock deadlocks the next method that takes it. The comment on
// noCopy in sync/cond.go says it "must not be embedded, due to the Lock and
// Unlock methods."
//
// Report only: the repair is a named field with Lock and Unlock declared over
// it, so the mutex's methods stay out of the type's method set, and the
// analyzer cannot choose the name. Every selector of the lock changes with the
// repair, in this package and in every importer.
//
// The README catalogs every check, exemption, and fix.
//
// The rule is adapted from go-critic's exposedSyncMutex.
package exposedsyncmutex

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "exposedsyncmutex",
	Doc:  "reports a sync.Mutex or sync.RWMutex embedded in a reachable type",
	Run:  run,
}

// reacher is the state of one run.
type reacher struct {
	pass     *analysis.Pass
	seen     map[types.Type]bool
	reported map[token.Pos]bool
}

func run(pass *analysis.Pass) (any, error) {
	// Nothing imports a main package, so no importer reaches its locks.
	if pass.Pkg.Name() == "main" {
		return nil, nil
	}
	r := &reacher{
		pass:     pass,
		seen:     make(map[types.Type]bool),
		reported: make(map[token.Pos]bool),
	}
	for _, file := range pass.Files {
		// A test file is not part of the package an importer compiles.
		if strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				r.declare(decl)
			case *ast.FuncDecl:
				r.result(decl)
			}
		}
	}
	return nil, nil
}

// declare reaches the types that a type or variable declaration gives an
// exported name.
func (r *reacher) declare(decl *ast.GenDecl) {
	if decl.Tok != token.TYPE && decl.Tok != token.VAR {
		return
	}
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			if !ast.IsExported(spec.Name.Name) {
				continue
			}
			// Reach the declared type, so the methods declared on it are read.
			tn, ok := r.pass.TypesInfo.Defs[spec.Name].(*types.TypeName)
			if ok {
				r.reach(tn.Type())
			}
		case *ast.ValueSpec:
			for _, name := range spec.Names {
				if !ast.IsExported(name.Name) {
					continue
				}
				if v, ok := r.pass.TypesInfo.Defs[name].(*types.Var); ok {
					r.reach(v.Type())
				}
			}
		}
	}
}

// result reaches the result types of an exported function.
func (r *reacher) result(decl *ast.FuncDecl) {
	if decl.Recv != nil || !ast.IsExported(decl.Name.Name) {
		return
	}
	fn, ok := r.pass.TypesInfo.Defs[decl.Name].(*types.Func)
	if !ok {
		return
	}
	r.reach(fn.Type())
}

// reach walks what typ leads to, reporting each embedded lock.
//
// A type another package declares is not read.
func (r *reacher) reach(typ types.Type) {
	typ = deref(typ)
	if named, ok := typ.(*types.Named); ok {
		if named.Obj().Pkg() != r.pass.Pkg {
			return
		}
		if r.seen[named] {
			return
		}
		r.seen[named] = true
		r.methods(named)
		// The instantiated struct, so that a field of a type parameter is the
		// type argument the declaration supplied.
		typ = named.Underlying()
	}
	if r.seen[typ] {
		return
	}
	r.seen[typ] = true
	switch typ := typ.(type) {
	case *types.Struct:
		r.fields(typ)
	case *types.Signature:
		for result := range typ.Results().Variables() {
			r.reach(result.Type())
		}
	case *types.Interface:
		for method := range typ.Methods() {
			if method.Exported() {
				r.reach(method.Type())
			}
		}
	case *types.Slice:
		r.reach(typ.Elem())
	case *types.Array:
		r.reach(typ.Elem())
	case *types.Map:
		r.reach(typ.Key())
		r.reach(typ.Elem())
	case *types.Chan:
		r.reach(typ.Elem())
	case *types.Pointer:
		r.reach(typ.Elem())
	}
}

// methods reaches the result types of the methods an importer can call on
// named, including the ones promotion adds.
func (r *reacher) methods(named *types.Named) {
	set := types.NewMethodSet(types.NewPointer(named))
	for method := range set.Methods() {
		if method.Obj().Exported() {
			r.reach(method.Type())
		}
	}
}

// fields reports the embeds in st, and follows the fields that reach another
// type.
func (r *reacher) fields(st *types.Struct) {
	for field := range st.Fields() {
		// A field declared in another package sits in a struct that package
		// writes, and the run over that package holds the embed.
		if field.Pkg() != r.pass.Pkg {
			continue
		}
		// A field the package does not export and does not embed is reached
		// only through a name an importer cannot write.
		if !field.Embedded() && !field.Exported() {
			continue
		}
		if field.Embedded() && r.carriesLock(field.Type()) {
			r.report(field)
			continue
		}
		r.reach(field.Type())
	}
}

// carriesLock reports whether an embed of typ hands an importer a lock: typ is
// one itself, or a type another package declares whose Lock and Unlock are
// sync's. A type this package declares is walked instead, so that the embed
// inside it is the line reported.
func (r *reacher) carriesLock(typ types.Type) bool {
	if isSyncLock(typ) {
		return true
	}
	named, ok := deref(typ).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg() == r.pass.Pkg {
		return false
	}
	// A pointer to an interface has no methods, so an interface is read as it
	// stands.
	set := types.NewMethodSet(types.Type(named))
	if _, ok := named.Underlying().(*types.Struct); ok {
		set = types.NewMethodSet(types.NewPointer(named))
	}
	lock := set.Lookup(nil, "Lock")
	if lock == nil || !syncMethod(lock.Obj()) {
		return false
	}
	unlock := set.Lookup(nil, "Unlock")
	return unlock != nil && syncMethod(unlock.Obj())
}

// syncMethod reports whether obj is a method the sync package declares.
func syncMethod(obj types.Object) bool {
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "sync"
}

// qualifier names a type from another package by its package name, so a
// message reads foreign.Base rather than the import path.
func qualifier(pass *analysis.Pass) types.Qualifier {
	return func(p *types.Package) string {
		if p == pass.Pkg {
			return ""
		}
		return p.Name()
	}
}

// report writes a diagnostic on the field that embeds a lock. An instantiation
// has fields of its own at the positions of the declaration, so a position is
// reported once.
func (r *reacher) report(field *types.Var) {
	if r.reported[field.Pos()] {
		return
	}
	r.reported[field.Pos()] = true
	name := types.TypeString(field.Type(), qualifier(r.pass))
	r.pass.Report(analysis.Diagnostic{
		Pos:     field.Pos(),
		Message: "avoid embedding " + name + "; consider a named field",
	})
}

// deref returns typ with the pointers and the aliases around it stripped.
func deref(typ types.Type) types.Type {
	for {
		typ = types.Unalias(typ)
		ptr, ok := typ.(*types.Pointer)
		if !ok {
			return typ
		}
		typ = ptr.Elem()
	}
}

// isSyncLock reports whether typ is a sync.Mutex, a sync.RWMutex, or a pointer
// to one.
func isSyncLock(typ types.Type) bool {
	named, ok := deref(typ).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() == nil || obj.Pkg().Path() != "sync" {
		return false
	}
	switch obj.Name() {
	case "Mutex", "RWMutex":
		return true
	}
	return false
}
