// Package jsonrecv provides an analysis.Analyzer that reports MarshalJSON on a
// pointer receiver that does not need the pointer and UnmarshalJSON on a value
// receiver that writes directly to its copy.
//
// The spec's Method sets section says the method set of T holds the methods
// declared with receiver T, and the method set of *T also holds those declared
// with receiver *T. A pointer receiver is therefore in the method set of *T
// only, and a value of T does not implement json.Marshaler.
//
// The v1 Marshal in encoding/json takes the address of an addressable value
// and calls the method on that address. A value that is not addressable, such
// as a map element or a value held in an interface, is encoded without it. The
// encoding/json/v2 documentation says that Marshal "ensures that a value is
// always addressable (by copying the value if necessary)", so it calls the
// method either way.
//
// A value receiver works on a copy; Effective Go's "Pointers vs. Values"
// section says that a call on a value "would cause the method to receive a
// copy of the value, so any modifications would be discarded." A write to the
// receiver or to a field of it lands in that copy, and the value encoding/json
// decodes into keeps its old fields. The write is lost to the caller even when
// the method reads it back. A method that only reads its input is not
// reported. A write made through a call or a local alias is not followed, so a
// value receiver that writes in one is left alone. A write through a pointer,
// slice or map the receiver holds is not reported either.
//
// A pointer receiver is left alone when the method writes to the receiver or a
// field of it, or compares it with the nil identifier using == or !=, because
// a value receiver would lose the write and cannot test for a nil receiver. A
// write through a pointer, slice or map the receiver holds reaches shared
// storage, so a value receiver would keep it and the method is still reported.
// A pointer receiver is also left alone when the receiver's type contains a
// field of a synchronization type, including an array of one, because a value
// receiver would copy it. A defined type over a synchronization type is not
// recognized. A call through a pointer-receiver method on the receiver or the
// address of a field of it, or one that receives the receiver or a slice of an
// array in it, is treated as a write. A call through a pointer field or an
// embedded pointer, a call that receives the pointer receiver's address, and a
// call that reaches the value another way are not followed. A local alias is
// not followed. A pointer receiver on a type that is expensive to copy is
// still reported; //ignore on the method records that choice.
//
// A method is reported only when its signature matches the interface method it
// is named for: () ([]byte, error) for MarshalJSON, and ([]byte) error for
// UnmarshalJSON. A method with another signature does not implement the
// interface, and encoding/json does not call it.
//
// Report only. For MarshalJSON, a value receiver copies the receiver on every
// call; for UnmarshalJSON, a pointer receiver takes the method out of T's
// method set, so a caller that holds a T in an interface may stop satisfying
// json.Unmarshaler. Neither change is safe without knowing how the method uses
// the receiver.
//
// The rule is adapted from revive's marshal-receiver, which reports every
// MarshalJSON, MarshalText, MarshalYAML and unmarshal counterpart by receiver
// kind alone. This narrows it to JSON, to MarshalJSON on a pointer receiver,
// and to an UnmarshalJSON value receiver that writes to its copy.
package jsonrecv

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "jsonrecv",
	Doc:  "reports MarshalJSON on a pointer receiver that does not need the pointer and a direct UnmarshalJSON write to a value receiver",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			check(pass, fn)
		}
	}
	return nil, nil
}

// check reports fn when a pointer receiver leaves MarshalJSON out of T's
// method set, or a value receiver loses an UnmarshalJSON write.
func check(pass *analysis.Pass, fn *ast.FuncDecl) {
	obj, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func)
	if !ok {
		return
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return
	}
	_, pointer := sig.Recv().Type().(*types.Pointer)
	w := &walker{
		pass:    pass,
		body:    fn.Body,
		recv:    receiver(pass, fn),
		pointer: pointer,
	}
	switch fn.Name.Name {
	case "MarshalJSON":
		if !pointer || !marshalJSON(sig) {
			return
		}
		base := sig.Recv().Type()
		if p, ok := base.(*types.Pointer); ok {
			base = p.Elem()
		}
		if w.writesValue() || w.comparesNil() {
			return
		}
		if holdsSync(base) {
			return
		}
		pass.Reportf(fn.Recv.Pos(), "MarshalJSON has a pointer receiver; a value of the type does not implement json.Marshaler")
	case "UnmarshalJSON":
		if pointer || !unmarshalJSON(sig) {
			return
		}
		if !w.writesValue() {
			return
		}
		pass.Reportf(fn.Recv.Pos(), "UnmarshalJSON has a value receiver; its writes land in a copy")
	}
}

// receiver returns the method's receiver variable, or nil when the receiver is
// unnamed and the body cannot refer to it.
func receiver(pass *analysis.Pass, fn *ast.FuncDecl) *types.Var {
	if len(fn.Recv.List[0].Names) == 0 {
		return nil
	}
	recv, _ := pass.TypesInfo.Defs[fn.Recv.List[0].Names[0]].(*types.Var)
	return recv
}

// marshalJSON reports whether sig is the signature of json.Marshaler.
func marshalJSON(sig *types.Signature) bool {
	if sig.Variadic() || sig.Params().Len() != 0 || sig.Results().Len() != 2 {
		return false
	}
	return isByteSlice(sig.Results().At(0).Type()) &&
		isError(sig.Results().At(1).Type())
}

// unmarshalJSON reports whether sig is the signature of json.Unmarshaler.
func unmarshalJSON(sig *types.Signature) bool {
	if sig.Variadic() || sig.Params().Len() != 1 || sig.Results().Len() != 1 {
		return false
	}
	return isByteSlice(sig.Params().At(0).Type()) &&
		isError(sig.Results().At(0).Type())
}

func isByteSlice(t types.Type) bool {
	return types.Identical(t, types.NewSlice(types.Typ[types.Uint8]))
}

func isError(t types.Type) bool {
	return types.Identical(t, types.Universe.Lookup("error").Type())
}

// walker reports the writes in one method body to the receiver's value.
type walker struct {
	pass    *analysis.Pass
	body    *ast.BlockStmt
	recv    *types.Var
	pointer bool
}

// writesValue reports whether the body writes storage in the receiver's value:
// the caller's value for a pointer receiver, or the copy for a value receiver.
// A pointer receiver is also left alone when a call may write through it,
// because a value receiver would not follow that call.
func (w *walker) writesValue() bool {
	if w.body == nil || w.recv == nil {
		return false
	}
	if writes(w.body, w.within) {
		return true
	}
	return w.pointer && w.calls()
}

// writes reports whether body writes to a target the predicate accepts.
func writes(body *ast.BlockStmt, target func(ast.Expr) bool) bool {
	if body == nil {
		return false
	}
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.AssignStmt:
			if slices.ContainsFunc(n.Lhs, target) {
				found = true
				return false
			}
		case *ast.IncDecStmt:
			if target(n.X) {
				found = true
			}
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				if target(n.Key) {
					found = true
				} else if n.Value != nil && target(n.Value) {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// calls reports whether a call may write the caller's value through a pointer
// receiver: a method call on it, or one that receives it, its address, or a
// slice of an array in it.
func (w *walker) calls() bool {
	var found bool
	ast.Inspect(w.body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			if w.methodWrites(sel) {
				found = true
				return false
			}
		}
		for _, arg := range call.Args {
			passes := w.isReceiver(arg) || w.isAddressOfWithin(arg)
			if passes || w.sliceOfValue(arg) {
				found = true
				break
			}
		}
		return !found
	})
	return found
}

// methodWrites reports whether a call through sel reaches a pointer-receiver
// method that is not promoted through an embedded pointer.
func (w *walker) methodWrites(sel *ast.SelectorExpr) bool {
	if !w.methodBase(sel.X) {
		return false
	}
	selection, ok := w.pass.TypesInfo.Selections[sel]
	if !ok || w.promotedThroughPointer(selection) {
		return false
	}
	fn, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	_, pointer := sig.Recv().Type().(*types.Pointer)
	return pointer
}

func (w *walker) methodBase(e ast.Expr) bool {
	if w.isReceiver(e) || w.isAddressOfWithin(e) {
		return true
	}
	if t := w.pass.TypesInfo.TypeOf(e); t != nil {
		if _, ok := t.Underlying().(*types.Pointer); ok {
			return false
		}
	}
	return w.within(e)
}

// promotedThroughPointer reports whether a promoted selection reaches its
// method through an embedded pointer.
func (*walker) promotedThroughPointer(sel *types.Selection) bool {
	index := sel.Index()
	if len(index) < 2 {
		return false
	}
	typ := sel.Recv()
	if p, ok := typ.Underlying().(*types.Pointer); ok {
		typ = p.Elem()
	}
	for _, field := range index[:len(index)-1] {
		s, ok := typ.Underlying().(*types.Struct)
		if !ok {
			return false
		}
		next := s.Field(field).Type()
		if _, ok := next.Underlying().(*types.Pointer); ok {
			return true
		}
		typ = next
	}
	return false
}

func (w *walker) sliceOfValue(e ast.Expr) bool {
	s, ok := ast.Unparen(e).(*ast.SliceExpr)
	return ok && w.indexInValue(s.X)
}

func (w *walker) isAddressOfWithin(e ast.Expr) bool {
	u, ok := ast.Unparen(e).(*ast.UnaryExpr)
	return ok && u.Op == token.AND && w.within(u.X)
}

// within reports whether e names storage in the receiver's value. A write
// through a pointer, slice or map the value holds reaches shared storage, so
// it is not within.
func (w *walker) within(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		return !w.pointer && w.pass.TypesInfo.ObjectOf(x) == w.recv
	case *ast.StarExpr:
		if w.isReceiver(x.X) {
			return w.pointer
		}
		if w.isAddressOf(x.X) {
			return w.within(x.X)
		}
		return false
	case *ast.UnaryExpr:
		if x.Op != token.AND {
			return false
		}
		return w.within(x.X)
	case *ast.SelectorExpr:
		sel, ok := w.pass.TypesInfo.Selections[x]
		if !ok {
			return false
		}
		if !w.withinBase(x.X) {
			return false
		}
		return w.fieldInValue(sel, x.X)
	case *ast.IndexExpr:
		return w.indexInValue(x.X)
	case *ast.SliceExpr:
		return w.indexInValue(x.X)
	}
	return false
}

// withinBase reports whether the base of a selector names the receiver's value
// or a value expression inside it.
func (w *walker) withinBase(e ast.Expr) bool {
	if w.isReceiver(e) {
		return true
	}
	return w.within(e)
}

// fieldInValue reports whether a selection names a field in the receiver's
// value. A field reached through an embedded pointer is shared, so a value
// receiver would keep that write.
func (w *walker) fieldInValue(sel *types.Selection, base ast.Expr) bool {
	if sel.Kind() != types.FieldVal {
		return false
	}
	typ := sel.Recv()
	if p, ok := typ.Underlying().(*types.Pointer); ok {
		switch {
		case w.isAddressOf(base):
			typ = p.Elem()
		case w.pointer && w.isReceiver(base):
			typ = p.Elem()
		default:
			return false
		}
	}
	index := sel.Index()
	for i, field := range index {
		s, ok := typ.Underlying().(*types.Struct)
		if !ok {
			return false
		}
		next := s.Field(field).Type()
		if i == len(index)-1 {
			return true
		}
		if _, ok := next.Underlying().(*types.Pointer); ok {
			return false
		}
		typ = next
	}
	return true
}

// indexInValue reports whether indexing or slicing base writes the receiver's
// value.
func (w *walker) indexInValue(base ast.Expr) bool {
	if s, ok := ast.Unparen(base).(*ast.SliceExpr); ok {
		return w.indexInValue(s.X)
	}
	if w.isPointerToArray(base) {
		switch x := ast.Unparen(base).(type) {
		case *ast.Ident:
			return w.pointer && w.isReceiver(x)
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				return w.within(x.X)
			}
		}
		return false
	}
	return w.isArray(base) && w.within(base)
}

func (*walker) isAddressOf(e ast.Expr) bool {
	u, ok := ast.Unparen(e).(*ast.UnaryExpr)
	return ok && u.Op == token.AND
}

func (w *walker) isArray(e ast.Expr) bool {
	t := w.pass.TypesInfo.TypeOf(e)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Array)
	return ok
}

func (w *walker) isPointerToArray(e ast.Expr) bool {
	t := w.pass.TypesInfo.TypeOf(e)
	if t == nil {
		return false
	}
	p, ok := t.Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	_, ok = p.Elem().Underlying().(*types.Array)
	return ok
}

// comparesNil reports whether the body compares the receiver with nil.
func (w *walker) comparesNil() bool {
	if w.body == nil || w.recv == nil {
		return false
	}
	var found bool
	ast.Inspect(w.body, func(n ast.Node) bool {
		if found {
			return false
		}
		bin, ok := n.(*ast.BinaryExpr)
		if !ok || (bin.Op != token.EQL && bin.Op != token.NEQ) {
			return true
		}
		left := isNil(bin.Y) && w.isReceiver(bin.X)
		right := isNil(bin.X) && w.isReceiver(bin.Y)
		if left || right {
			found = true
		}
		return !found
	})
	return found
}

func isNil(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "nil"
}

func (w *walker) isReceiver(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && w.pass.TypesInfo.ObjectOf(id) == w.recv
}

// holdsSync reports whether t holds a synchronization value that a copy would
// duplicate.
func holdsSync(t types.Type) bool {
	return holdsSyncSeen(t, make(map[types.Type]bool))
}

func holdsSyncSeen(t types.Type, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	t = types.Unalias(t)
	if syncValue(t) {
		return true
	}
	switch u := t.Underlying().(type) {
	case *types.Struct:
		for field := range u.Fields() {
			if holdsSyncSeen(field.Type(), seen) {
				return true
			}
		}
	case *types.Array:
		return holdsSyncSeen(u.Elem(), seen)
	}
	return false
}

func syncValue(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() == nil {
		return false
	}
	switch obj.Pkg().Path() {
	case "sync/atomic":
		return true
	case "sync":
		switch obj.Name() {
		case "Cond", "Map", "Mutex", "Once", "Pool", "RWMutex", "WaitGroup":
			return true
		}
	}
	return false
}
