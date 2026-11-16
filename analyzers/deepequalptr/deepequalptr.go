// Package deepequalptr provides an analysis.Analyzer that reports a
// reflect.DeepEqual call on a type with an Equal method and a pointer field.
//
// The reflect package's DeepEqual documentation says that pointer values are
// deeply equal "if they are equal using Go's == operator or if they point to
// deeply equal values" (https://pkg.go.dev/reflect#DeepEqual). DeepEqual does
// not call Equal. It compares the fields, and at a pointer field it compares
// the values the pointers point to. The two comparisons can therefore
// disagree.
//
//	type item struct {
//		meta *int
//	}
//
//	func (i item) Equal(other item) bool {
//		return i.meta == other.meta
//	}
//
//	func same(left, right item) bool {
//		return reflect.DeepEqual(left, right)
//	}
//
// The rule reports the call when both arguments have the same type, and that
// type, after its pointers are followed, has an Equal method on the type or a
// pointer to it. The method takes a parameter of its own receiver type and
// returns a boolean type. The compared type must be a struct or a pointer to
// one. A pointer field is read through struct fields, array and slice
// elements, and map values. The diagnostic names the path to the first
// non-blank pointer field in declaration order. Pointers at the top of the
// compared type are dereferenced before its fields are read.
//
// An array, a slice, or a map field is read through its element or value type,
// and the path adds [] for each container. A zero-length array field is not
// read. A pointer field that is unexported and comes from another package is
// named only as a pointer field. A channel, function, or interface field is
// not read. A field whose type is a type parameter is not read as a pointer,
// even when its constraint admits only pointer types. A type with no reachable
// pointer field, and a type that is not a struct or a pointer to one, is left
// alone. A type is left alone when its Equal method takes another type or is
// promoted from an embedded field. A type whose Equal method does not return a
// boolean type is left alone as well. A same-package Equal method that
// dereferences the first direct pointer field is left alone; the rule checks
// the method body for that field. The rule does not report a call whose
// arguments have different static types; the dynamic type behind an interface
// is not known. A call through a function value is left alone as well; only a
// direct call to reflect.DeepEqual is matched. A test file is skipped
// entirely.
//
// An Equal method that reads the same pointee as DeepEqual is still reported
// when the method is in another package; the rule checks only the method
// signature there.
//
// Report only: the repair calls Equal with the dereferences or addresses the
// receiver requires, and changes behavior. Bind reflect.DeepEqual to a
// variable when the structural comparison is intended.
//
// The README catalogs every check and exemption. The rule is inspired by
// staticcheck's SA1021, which reports a net.IP comparison by its bytes instead
// of net.IP.Equal.
package deepequalptr

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "deepequalptr",
	Doc:  "reports DeepEqual on a type with an Equal method and a pointer field",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isDeepEqual(pass, call) || len(call.Args) != 2 {
				return true
			}
			check(pass, call)
			return true
		})
	}
	return nil, nil
}

// isDeepEqual reports whether call calls reflect.DeepEqual.
func isDeepEqual(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == "reflect" &&
		fn.Name() == "DeepEqual"
}

// check reports call when its arguments have the same type, a pointer field is
// reachable from that type, and the type has an Equal method.
func check(pass *analysis.Pass, call *ast.CallExpr) {
	var (
		typ   = pass.TypesInfo.TypeOf(call.Args[0])
		other = pass.TypesInfo.TypeOf(call.Args[1])
	)
	if typ == nil || !types.Identical(typ, other) {
		return
	}
	field, fieldVar, ok := pointerField(typ)
	if !ok {
		return
	}
	recv, fn, ok := equalReceiver(pass, typ)
	if !ok || equalDereferences(pass, fn, field) {
		return
	}
	msg := "reflect.DeepEqual does not call " + equalName(pass, recv) +
		"; a pointer field is followed to its pointee"
	if fieldVar.Exported() || fieldVar.Pkg() == pass.Pkg {
		msg = "reflect.DeepEqual does not call " + equalName(pass, recv) +
			"; pointer field " + field + " is followed to its pointee"
	}
	pass.Report(analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: msg,
	})
}

// equalReceiver returns the receiver type of the Equal method that applies to
// t, after any pointer is followed. The method takes an argument of its own
// receiver type and returns a boolean type.
func equalReceiver(pass *analysis.Pass, t types.Type) (types.Type, *types.Func, bool) {
	var (
		base      = baseType(t)
		obj, _, _ = types.LookupFieldOrMethod(base, true, pass.Pkg, "Equal")
		fn, ok    = obj.(*types.Func)
	)
	if !ok {
		return nil, nil, false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil || sig.Params().Len() != 1 {
		return nil, nil, false
	}
	if sig.Results().Len() != 1 {
		return nil, nil, false
	}
	recv := sig.Recv().Type()
	if !types.Identical(recv, sig.Params().At(0).Type()) {
		return nil, nil, false
	}
	if !types.Identical(recv, base) {
		ptr, ok := recv.Underlying().(*types.Pointer)
		if !ok || !types.Identical(ptr.Elem(), base) {
			return nil, nil, false
		}
	}
	basic, ok := sig.Results().At(0).Type().Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsBoolean == 0 {
		return nil, nil, false
	}
	return recv, fn, true
}

// equalDereferences reports whether a same-package Equal method dereferences
// the direct pointer field named field.
func equalDereferences(pass *analysis.Pass, fn *types.Func, name string) bool {
	if name == "" || strings.ContainsAny(name, ".[") {
		return false
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			decl, ok := decl.(*ast.FuncDecl)
			if !ok || decl.Body == nil {
				continue
			}
			if pass.TypesInfo.Defs[decl.Name] != fn {
				continue
			}
			var found bool
			ast.Inspect(decl.Body, func(n ast.Node) bool {
				star, ok := n.(*ast.StarExpr)
				if !ok {
					return true
				}
				sel, ok := ast.Unparen(star.X).(*ast.SelectorExpr)
				if ok && sel.Sel.Name == name {
					found = true
					return false
				}
				return true
			})
			return found
		}
	}
	return false
}

// equalName returns the text that names the Equal method with receiver t.
func equalName(pass *analysis.Pass, t types.Type) string {
	t = types.Unalias(t)
	name := types.TypeString(t, func(pkg *types.Package) string {
		if pkg == pass.Pkg {
			return ""
		}
		return pkg.Name()
	})
	if _, ok := t.Underlying().(*types.Pointer); ok {
		return "(" + name + ").Equal"
	}
	return name + ".Equal"
}

// baseType returns the type reached from t by following pointers.
func baseType(t types.Type) types.Type {
	seen := make(map[types.Type]struct{})
	t = types.Unalias(t)
	for {
		ptr, ok := t.Underlying().(*types.Pointer)
		if !ok {
			return t
		}
		if _, ok := seen[t]; ok {
			return t
		}
		seen[t] = struct{}{}
		t = types.Unalias(ptr.Elem())
	}
}

// pointerField returns the path to the first pointer field reachable from t
// through struct fields, array and slice elements, and map values, in
// declaration order. Pointers at the top of t are followed first, since the
// arguments themselves may be pointers.
func pointerField(t types.Type) (string, *types.Var, bool) {
	t = baseType(t)
	if _, ok := t.Underlying().(*types.Struct); !ok {
		return "", nil, false
	}
	return fieldPointer(t, "", nil, make(map[types.Type]struct{}))
}

// fieldPointer returns the path to the first pointer in t and the field that
// holds it. An array, a slice, or a map adds [] to the path. The path is empty
// only at the top, which is a struct.
func fieldPointer(t types.Type, path string, field *types.Var, seen map[types.Type]struct{}) (string, *types.Var, bool) {
	t = types.Unalias(t)
	if t == nil {
		return "", nil, false
	}
	if _, ok := seen[t]; ok {
		return "", nil, false
	}
	seen[t] = struct{}{}
	switch t := t.Underlying().(type) {
	case *types.Pointer:
		if path == "" {
			return "", nil, false
		}
		return path, field, true
	case *types.Struct:
		for next := range t.Fields() {
			if next.Name() == "_" {
				continue
			}
			name := next.Name()
			if path != "" {
				name = path + "." + name
			}
			found, owner, ok := fieldPointer(next.Type(), name, next, seen)
			if ok {
				return found, owner, true
			}
		}
	case *types.Array:
		if t.Len() == 0 {
			return "", nil, false
		}
		return fieldPointer(t.Elem(), path+"[]", field, seen)
	case *types.Slice:
		return fieldPointer(t.Elem(), path+"[]", field, seen)
	case *types.Map:
		return fieldPointer(t.Elem(), path+"[]", field, seen)
	}
	return "", nil, false
}
