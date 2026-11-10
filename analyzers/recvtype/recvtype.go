// Package recvtype provides an analysis.Analyzer that reports a type whose
// methods mix pointer and value receivers.
//
// A value receiver works on a copy, so a method that changes the value
// through one changes nothing the caller can see. One kind for the whole type
// also settles which methods a value has. The Go review comments state the
// rule as "Don't mix receiver types".
package recvtype

import (
	"go/ast"

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
	if index, ok := typ.(*ast.IndexExpr); ok {
		typ = index.X
	}
	id, ok := typ.(*ast.Ident)
	if !ok {
		return "", false, false
	}
	return id.Name, ptr, true
}
