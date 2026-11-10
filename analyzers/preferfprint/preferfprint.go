// Package preferfprint provides an analysis.Analyzer that reports a formatted
// string written to a writer.
//
// w.WriteString(fmt.Sprintf(...)) builds the string and hands it over.
// fmt.Fprintf(w, ...) formats straight into the writer and allocates nothing
// in between. The Write([]byte(...)) form is the same shape. Like go-critic's
// preferFprint.
package preferfprint

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "preferfprint",
	Doc:  "reports a formatted string written to a writer",
	Run:  run,
}

// formatters maps a Sprint family member to the Fprint member that writes it.
var formatters = map[string]string{
	"Sprintf":  "Fprintf",
	"Sprint":   "Fprint",
	"Sprintln": "Fprintln",
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			writer, value, ok := written(pass, call)
			if !ok {
				return true
			}
			sprint, ok := formatted(pass, value)
			if !ok {
				return true
			}
			sel := sprint.Fun.(*ast.SelectorExpr)
			text := types.ExprString(sel.X) + "." + formatters[sel.Sel.Name] +
				"(" + types.ExprString(writer) + arguments(sprint.Args) + ")"
			pass.Report(analysis.Diagnostic{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "formatted string written to a writer; use Fprint",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Format into the writer",
					TextEdits: []analysis.TextEdit{{
						Pos:     call.Pos(),
						End:     call.End(),
						NewText: []byte(text),
					}},
				}},
			})
			return true
		})
	}
	return nil, nil
}

// written returns the writer call writes to and the value it writes, when call
// is one of the ways to write a string.
func written(pass *analysis.Pass, call *ast.CallExpr) (writer, value ast.Expr, ok bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, nil, false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if ok && pkg.Imported().Path() == "io" && sel.Sel.Name == "WriteString" {
		if len(call.Args) != 2 {
			return nil, nil, false
		}
		return call.Args[0], call.Args[1], true
	}
	if sel.Sel.Name != "WriteString" && sel.Sel.Name != "Write" {
		return nil, nil, false
	}
	if len(call.Args) != 1 || !isWriter(pass, sel.X) {
		return nil, nil, false
	}
	return sel.X, call.Args[0], true
}

// formatted returns the Sprint family call in value, through a byte slice
// conversion when the writer takes bytes.
func formatted(pass *analysis.Pass, value ast.Expr) (*ast.CallExpr, bool) {
	conv, isCall := value.(*ast.CallExpr)
	if isCall && isBytes(conv.Fun) && len(conv.Args) == 1 {
		value = conv.Args[0]
	}
	call, ok := value.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || formatters[sel.Sel.Name] == "" {
		return nil, false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil, false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok || pkg.Imported().Path() != "fmt" {
		return nil, false
	}
	return call, true
}

// arguments renders args as the tail of an argument list.
func arguments(args []ast.Expr) string {
	var b strings.Builder
	for _, arg := range args {
		b.WriteString(", " + types.ExprString(arg))
	}
	return b.String()
}

// isWriter reports whether e implements io.Writer, which Fprintf needs.
func isWriter(pass *analysis.Pass, e ast.Expr) bool {
	typ := pass.TypesInfo.TypeOf(e)
	if typ == nil {
		return false
	}
	writer := types.NewInterfaceType([]*types.Func{
		types.NewFunc(0, nil, "Write", types.NewSignatureType(nil, nil, nil,
			types.NewTuple(types.NewVar(0, nil, "", types.NewSlice(
				types.Typ[types.Byte],
			))),
			types.NewTuple(
				types.NewVar(0, nil, "", types.Typ[types.Int]),
				types.NewVar(0, nil, "", errorType()),
			),
			false,
		)),
	}, nil)
	writer.Complete()
	return types.Implements(typ, writer)
}

// errorType returns the predeclared error type.
func errorType() types.Type {
	return types.Universe.Lookup("error").Type()
}

// isBytes reports whether e is the type []byte.
func isBytes(e ast.Expr) bool {
	array, ok := e.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return false
	}
	elem, ok := array.Elt.(*ast.Ident)
	return ok && elem.Name == "byte"
}
