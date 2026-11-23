// Package recvtype provides an analysis.Analyzer that reports a type whose
// methods mix pointer and value receivers.
//
// A value receiver works on a copy, so a method that changes the value
// through one changes nothing the caller can see. One kind for the whole type
// also settles which methods a value has. The Go review comments state the
// rule as "Don't mix receiver types".
//
// A pointer-receiver method whose signature is one the standard decoding
// interfaces state is exempt: UnmarshalJSON, UnmarshalText, UnmarshalBinary,
// UnmarshalXML, GobDecode and Scan all write into the value, so the interface
// forces the pointer and the receiver says nothing about the rest of the type.
// The name and the signature are matched together, so a method that merely
// begins with Unmarshal, or carries the name with another signature, is
// reported.
package recvtype

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "recvtype",
	Doc:  "reports a type whose methods mix pointer and value receivers",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	var (
		pointer  = make(map[string]bool)
		reported = make(map[string]struct{})
	)
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			typ, ptr, ok := receiver(fn)
			if _, done := reported[typ]; !ok || done {
				continue
			}
			// A decoder writes through the pointer it is handed and is
			// called only that way, so it does not settle the kind for
			// the rest of the type.
			if ptr && isDecoder(fn, pass) {
				continue
			}
			if kind, ok := pointer[typ]; ok && kind != ptr {
				pass.Reportf(fn.Recv.Pos(), "methods of %s mix pointer and value receivers", typ)
				reported[typ] = struct{}{}
				continue
			}
			pointer[typ] = ptr
		}
	}
	return nil, nil
}

// isDecoder reports whether fn states the name and the signature of one of the
// standard decoding interfaces. It does not look at the receiver: the caller
// asks only about pointer receivers.
func isDecoder(fn *ast.FuncDecl, pass *analysis.Pass) bool {
	obj, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func)
	if !ok {
		return false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		return false
	}
	params, results := sig.Params(), sig.Results()
	if results.Len() != 1 || !isError(results.At(0).Type()) {
		return false
	}
	switch fn.Name.Name {
	case "UnmarshalJSON", "UnmarshalText", "UnmarshalBinary", "GobDecode":
		return params.Len() == 1 && isByteSlice(params.At(0).Type())
	case "UnmarshalXML":
		return params.Len() == 2 &&
			isNamed(params.At(0).Type(), "encoding/xml", "Decoder", true) &&
			isNamed(params.At(1).Type(), "encoding/xml", "StartElement", false)
	case "Scan":
		return params.Len() == 1 && isEmptyInterface(params.At(0).Type())
	}
	return false
}

// isError reports whether t is the predeclared error type.
func isError(t types.Type) bool {
	return types.Identical(t, types.Universe.Lookup("error").Type())
}

// isByteSlice reports whether t is []byte.
func isByteSlice(t types.Type) bool {
	return types.Identical(t, types.NewSlice(types.Typ[types.Uint8]))
}

// isNamed reports whether t is the named type pkg.name, or a pointer to it
// when ptr is set.
func isNamed(t types.Type, pkg, name string, ptr bool) bool {
	if p, ok := t.(*types.Pointer); ok {
		if !ptr {
			return false
		}
		t = p.Elem()
	} else if ptr {
		return false
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == pkg && named.Obj().Name() == name
}

// isEmptyInterface reports whether t is an interface type with no methods.
func isEmptyInterface(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	return ok && iface.NumMethods() == 0
}

// receiver returns the name of the receiver's type and whether it is a
// pointer.
func receiver(fn *ast.FuncDecl) (_ string, _, _ bool) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", false, false
	}
	var (
		typ = fn.Recv.List[0].Type
		ptr bool
	)
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
		ptr = true
	}
	switch index := typ.(type) {
	case *ast.IndexExpr:
		typ = index.X
	case *ast.IndexListExpr:
		typ = index.X
	}
	id, ok := typ.(*ast.Ident)
	if !ok {
		return "", false, false
	}
	return id.Name, ptr, true
}
