// Package printbytes provides an analysis.Analyzer that reports a byte slice
// printed by a call that formats its operands the default way.
//
// The fmt package doc says Print "formats using the default formats for its
// operands". The default format of a slice prints its elements, so a byte
// slice reaches the output as the decimal value of each byte: printing
// []byte("hi") writes "[104 105]". Converting the operand to a string writes
// the text those bytes encode.
//
// The fix wraps the operand in a string conversion, which writes that text
// instead of the numbers. It is withheld where the name string resolves to
// another declaration.
//
// A format-driven call is not reported: the verb chooses the form, so
// fmt.Printf("%v", b) writes the numbers and %s writes the text.
//
// The README catalogs every check, exemption, and fix.
//
// The rule is adapted from staticcheck's QF1010, which reports a byte slice
// printed by a call and wraps it in the same conversion.
package printbytes

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/imports"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "printbytes",
	Doc:  "reports a byte slice printed with the default format",
	Run:  run,
}

// printers holds the functions that format their operands the default way,
// keyed by the package that declares them and the name it declares there.
// writer marks the calls whose first operand is where the text goes.
var printers = map[string]struct{ writer bool }{
	"fmt.Fprint":    {writer: true},
	"fmt.Fprintln":  {writer: true},
	"fmt.Print":     {},
	"fmt.Println":   {},
	"fmt.Sprint":    {},
	"fmt.Sprintln":  {},
	"log.Fatal":     {},
	"log.Fatalln":   {},
	"log.Panic":     {},
	"log.Panicln":   {},
	"log.Print":     {},
	"log.Println":   {},
	"testing.Error": {},
	"testing.Fatal": {},
	"testing.Log":   {},
	"testing.Skip":  {},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports every operand of call that prints as a byte slice.
func check(pass *analysis.Pass, call *ast.CallExpr) {
	for _, arg := range operands(pass, call) {
		typ := pass.TypesInfo.TypeOf(arg)
		if !byteSlice(typ) || formatsItself(typ) {
			continue
		}
		d := analysis.Diagnostic{
			Pos:     arg.Pos(),
			End:     arg.End(),
			Message: "byte slice formats as a list of numbers; convert it to a string",
		}
		// The conversion would call another declaration named string.
		decl := imports.Lookup(pass, "string", arg.Pos())
		if decl == types.Universe.Lookup("string") {
			d.SuggestedFixes = []analysis.SuggestedFix{wrap(arg)}
		}
		pass.Report(d)
	}
}

// operands returns the arguments a print call formats, and nothing when call
// is not one of them.
func operands(pass *analysis.Pass, call *ast.CallExpr) []ast.Expr {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil {
		return nil
	}
	printer, ok := printers[fn.Pkg().Path()+"."+fn.Name()]
	if !ok {
		return nil
	}
	args := call.Args
	if printer.writer && len(args) > 0 {
		args = args[1:]
	}
	return args
}

// byteSlice reports whether t is a slice of byte, which a string conversion
// accepts. The element's underlying type decides, so a slice of a defined byte
// type converts too.
func byteSlice(t types.Type) bool {
	if t == nil {
		return false
	}
	slice, ok := t.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	elem, ok := slice.Elem().Underlying().(*types.Basic)
	return ok && elem.Kind() == types.Byte
}

// formatsItself reports whether fmt asks t for its text instead of printing
// the elements. String, Error, and Format are the methods fmt looks for first,
// and converting the operand would replace the method's text with the text the
// bytes encode.
func formatsItself(t types.Type) bool {
	methods := types.NewMethodSet(t)
	for _, name := range []string{"Error", "Format", "String"} {
		sel := methods.Lookup(nil, name)
		if sel != nil && printsText(name, sel.Obj().Type()) {
			return true
		}
	}
	return false
}

// printsText reports whether a method of this name and type is one fmt calls
// to get a value's text: String and Error return a string, and Format takes
// fmt's state and a verb.
func printsText(name string, t types.Type) bool {
	sig, ok := t.(*types.Signature)
	if !ok {
		return false
	}
	params, results := sig.Params(), sig.Results()
	if name == "Format" {
		return params.Len() == 2 && results.Len() == 0 &&
			isState(params.At(0).Type()) &&
			types.Identical(params.At(1).Type(), types.Typ[types.Rune])
	}
	return params.Len() == 0 && results.Len() == 1 &&
		types.Identical(results.At(0).Type(), types.Typ[types.String])
}

// isState reports whether t is fmt.State, which fmt hands to Format.
func isState(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "fmt" &&
		obj.Name() == "State"
}

// wrap returns the fix that converts arg to a string. It changes the output,
// so the message says so.
func wrap(arg ast.Expr) analysis.SuggestedFix {
	return analysis.SuggestedFix{
		Message: "Convert the operand to a string (changes behavior)",
		TextEdits: []analysis.TextEdit{
			{Pos: arg.Pos(), End: arg.Pos(), NewText: []byte("string(")},
			{Pos: arg.End(), End: arg.End(), NewText: []byte(")")},
		},
	}
}
