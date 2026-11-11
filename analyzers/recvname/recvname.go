// Package recvname provides an analysis.Analyzer that reports a method
// receiver whose name does not match the other methods of its type.
//
// A receiver appears on almost every line of every method of its type, so
// writers keep it short and keep it the same. A type whose methods use c, cl
// and client in turn makes a reader check which value each method works on.
package recvname

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "recvname",
	Doc:  "reports a receiver name that differs from its type's other methods",
	Run:  run,
}

// banned are the receiver names that belong to another language's idiom.
var banned = map[string]struct{}{"this": {}, "self": {}, "me": {}}

func run(pass *analysis.Pass) (any, error) {
	var (
		// counts holds every name each type uses, and where each use is,
		// so the name a type mostly uses can be told from the odd one.
		counts = make(map[string]map[string]int)
		uses   = make(map[string][]use)
	)
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			id, typ, ok := receiver(fn)
			if !ok {
				continue
			}
			if _, ok := banned[id.Name]; ok {
				pass.Reportf(
					id.Pos(),
					"receiver %q; use the type's initial", id.Name,
				)
				continue
			}
			if counts[typ] == nil {
				counts[typ] = make(map[string]int)
			}
			counts[typ][id.Name]++
			uses[typ] = append(uses[typ], use{id, id.Name})
		}
	}
	for typ, byName := range counts {
		if len(byName) < 2 {
			continue
		}
		// The name a type mostly uses is the one to keep, so the odd
		// method is the one reported.
		want := majority(byName)
		for _, u := range uses[typ] {
			if u.name == want {
				continue
			}
			pass.Reportf(u.id.Pos(),
				"receiver name %q differs from %q on %s",
				u.name, want, typ,
			)
		}
	}
	return nil, nil
}

// use is one receiver name and where it was written.
type use struct {
	id   *ast.Ident
	name string
}

// majority returns the name byName holds most often, breaking a tie by the
// name that sorts first so the answer does not depend on file order.
func majority(byName map[string]int) (want string) {
	var most int
	for name, n := range byName {
		if n > most || (n == most && name < want) {
			want, most = name, n
		}
	}
	return want
}

// receiver returns the receiver's name and the name of its type.
func receiver(fn *ast.FuncDecl) (*ast.Ident, string, bool) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return nil, "", false
	}
	field := fn.Recv.List[0]
	if len(field.Names) == 0 || field.Names[0].Name == "_" {
		return nil, "", false
	}
	typ := typeName(field.Type)
	if typ == "" {
		return nil, "", false
	}
	return field.Names[0], typ, true
}

// typeName returns the name of the type in a receiver, dropping the pointer
// and any type arguments.
func typeName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return typeName(e.X)
	case *ast.IndexExpr:
		return typeName(e.X)
	case *ast.Ident:
		return e.Name
	}
	return ""
}
