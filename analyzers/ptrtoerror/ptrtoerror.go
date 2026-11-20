// Package ptrtoerror provides an analysis.Analyzer that reports a concrete
// error type that a package converts to error both as a value and as a
// pointer.
//
// Vendored from gopls' ptrtoerror (gopls/internal/analysis/ptrtoerror,
// golang.org/x/tools/gopls v0.24.0), which is internal and cannot be imported.
// The rule is upstream's; the code is adapted to this repository's helpers.
//
// A type assertion tests for the form it names. A package that writes both
// leaves the caller no form that always matches:
//
//	type mixed struct{ error }
//
//	func first(m mixed) error   { return m }
//	func second(m *mixed) error { return m }
//
// A conversion in either form is reported, and each report names one
// conversion in the other form. Neither report carries a fix, because
// rewriting a conversion changes which callers match: the form is the author's
// choice.
//
// A type that implements error both ways and is never converted to error in
// the package that declares it is reported once, at its name. Nothing in the
// package says which form was meant, so the fix declares the value form, which
// compiles because the type itself implements error.
//
//	var _ error = *new(mixed)
//
// When a package converts its own exported type to error, that conversion
// decides the form for the packages that import it. A conversion of the other
// form in another package is reported with the file, line and column where the
// form was decided.
//
// An alias is not reported at its own declaration, and an interface type and
// an unnamed type are never reported. A generic type takes no fix, because its
// type parameters are not in scope where a declaration would go.
package ptrtoerror

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name:      "ptrtoerror",
	Doc:       "reports a concrete error type converted to error both as a value and as a pointer",
	FactTypes: []analysis.Fact{(*isErrorFact)(nil)},
	Run:       run,
}

// forms holds the conversions of one local type to error, split by the form
// the conversion writes.
type forms struct{ value, pointer []conversion }

func run(pass *analysis.Pass) (any, error) {
	locals := make(map[*types.TypeName]*forms)
	for _, conv := range conversions(pass) {
		if !conv.toError() {
			continue
		}
		named, pointer := concreteError(conv.from)
		if named == nil {
			continue
		}
		tname := named.Obj()
		if tname.Pkg() != pass.Pkg {
			reportImported(pass, named, pointer, conv)
			continue
		}
		f := locals[tname]
		if f == nil {
			f = new(forms)
			locals[tname] = f
		}
		if pointer {
			f.pointer = append(f.pointer, conv)
		} else {
			f.value = append(f.value, conv)
		}
	}

	// Every conversion has been read, so each declaration can decide the form
	// of the type it names.
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if decl, ok := n.(*ast.GenDecl); ok {
				reportDecl(pass, decl, locals)
			}
			return true
		})
	}
	return nil, nil
}

// reportDecl reports the form of each type a declaration names: a type the
// package converts to error is reported when it writes both forms, an exported
// one records the form it writes, and a type converted in neither form is
// reported as ambiguous.
func reportDecl(pass *analysis.Pass, decl *ast.GenDecl, locals map[*types.TypeName]*forms) {
	for _, spec := range decl.Specs {
		tspec, ok := spec.(*ast.TypeSpec)
		if !ok || tspec.Assign.IsValid() {
			continue // an alias names a type declared elsewhere
		}
		tname, ok := pass.TypesInfo.Defs[tspec.Name].(*types.TypeName)
		if !ok {
			continue
		}
		f := locals[tname]
		switch {
		case f == nil:
			reportAmbiguous(pass, decl, tname)
		case f.value != nil && f.pointer != nil:
			reportForm(pass, tname.Name(), f.pointer, f.value[0], "value")
			reportForm(pass, tname.Name(), f.value, f.pointer[0], "pointer")
		case tname.Exported():
			exportForm(pass, tname, f)
		}
	}
}

// toError reports whether a conversion assigns to the error interface.
func (c conversion) toError() bool {
	errorType := types.Universe.Lookup("error").Type()
	return types.IsInterface(c.to) && types.AssignableTo(c.to, errorType)
}

// concreteError returns the named type a conversion writes and whether it
// writes it as a pointer. It returns nil for a type that is not a concrete
// named type, as an interface or an unnamed struct is.
func concreteError(t types.Type) (*types.Named, bool) {
	ptr, pointer := types.Unalias(t).(*types.Pointer)
	if pointer {
		t = ptr.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || types.IsInterface(named) {
		return nil, false
	}
	return named, pointer
}

// reportForm reports every conversion in convs, naming one conversion in the
// other form.
func reportForm(pass *analysis.Pass, name string, convs []conversion, other conversion, otherForm string) {
	for _, conv := range convs {
		pass.Report(analysis.Diagnostic{
			Pos:     conv.expr.Pos(),
			End:     conv.expr.End(),
			Message: fmt.Sprintf("%s is converted to error both as a value and as a pointer", name),
			Related: []analysis.RelatedInformation{{
				Pos:     other.expr.Pos(),
				End:     other.expr.End(),
				Message: fmt.Sprintf("converted as a %s here", otherForm),
			}},
		})
	}
}

// exportForm records the form an exported type is converted in, so a package
// that imports it can compare.
func exportForm(pass *analysis.Pass, tname *types.TypeName, f *forms) {
	first, pointer := f.value, false
	if f.pointer != nil {
		first, pointer = f.pointer, true
	}
	pass.ExportObjectFact(tname, &isErrorFact{
		Pointer: pointer,
		Where:   place(pass.Fset, first[0].expr.Pos()),
	})
}

// place names a position in the file it is written in, ignoring any line
// directive.
func place(fset *token.FileSet, pos token.Pos) string {
	var (
		at   = fset.PositionFor(pos, false)
		name = filepath.Base(at.Filename)
	)
	return fmt.Sprintf("%s:%d:%d", name, at.Line, at.Column)
}

// reportImported reports a conversion that disagrees with the form the
// package declaring the type chose.
func reportImported(pass *analysis.Pass, named *types.Named, pointer bool, conv conversion) {
	var (
		tname = named.Obj()
		fact  isErrorFact
	)
	if !pass.ImportObjectFact(tname, &fact) || fact.Pointer == pointer {
		return
	}
	var message string
	if pointer {
		message = fmt.Sprintf(
			"conversion of *%s to error, but package %q uses %s (sans pointer) as an error (e.g. at %s)",
			types.TypeString(named, (*types.Package).Name),
			tname.Pkg().Path(), tname.Name(), fact.Where,
		)
	} else {
		message = fmt.Sprintf(
			"conversion of %s to error, but package %q uses pointer *%s as an error (e.g. at %s)",
			types.TypeString(named, (*types.Package).Name),
			tname.Pkg().Path(), tname.Name(), fact.Where,
		)
	}
	pass.Report(analysis.Diagnostic{
		Pos:     conv.expr.Pos(),
		End:     conv.expr.End(),
		Message: message,
	})
}

// reportAmbiguous reports a type that implements error both ways and that the
// package never converts to error.
func reportAmbiguous(pass *analysis.Pass, decl *ast.GenDecl, tname *types.TypeName) {
	if !implementsError(tname) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:            tname.Pos(),
		End:            tname.Pos() + token.Pos(len(tname.Name())),
		Message:        fmt.Sprintf("both %[1]s and *%[1]s implement the error interface, making the intent ambiguous", tname.Name()),
		SuggestedFixes: declarationFix(pass, decl, tname),
	})
}

// declarationFix declares the value form as the one a type is converted to
// error in. A generic type takes no fix, because its type parameters are not
// in scope where the declaration goes.
func declarationFix(pass *analysis.Pass, decl *ast.GenDecl, tname *types.TypeName) []analysis.SuggestedFix {
	if generic(tname) {
		return nil
	}
	pos, ok := insertAt(pass, decl.End())
	if !ok {
		return nil
	}
	return []analysis.SuggestedFix{{
		Message: fmt.Sprintf("Declare that %s implements error", tname.Name()),
		TextEdits: []analysis.TextEdit{{
			Pos: pos,
			End: pos,
			NewText: fmt.Appendf(
				nil, "\nvar _ error = *new(%s)\n",
				tname.Name(),
			),
		}},
	}}
}

// generic reports whether a type takes type parameters, which are not in scope
// in a package-level declaration.
func generic(tname *types.TypeName) bool {
	named, ok := tname.Type().(*types.Named)
	return ok && named.TypeParams().Len() > 0
}

// insertAt returns where to write a declaration that follows pos: the start of
// the next line when the rest of pos's line is blank or a comment, and pos
// itself when code follows, as the brace that closes a function does.
func insertAt(pass *analysis.Pass, pos token.Pos) (token.Pos, bool) {
	tf := pass.Fset.File(pos)
	if tf == nil {
		return token.NoPos, false
	}
	src, err := pass.ReadFile(tf.Name())
	if err != nil || tf.Size() != len(src) {
		return token.NoPos, false
	}
	at := tf.Offset(pos)
	next := source.End(src, at)
	rest := bytes.TrimSpace(src[at:next])
	if len(rest) > 0 && !bytes.HasPrefix(rest, []byte("//")) {
		return pos, true
	}
	return tf.Pos(next), true
}

// implementsError reports whether a type is a form of error that a package can
// choose between. An interface type is not one: the report says both it and a
// pointer to it implement error, and a pointer to an interface implements
// nothing.
func implementsError(tname *types.TypeName) bool {
	var (
		t         = tname.Type()
		errorType = types.Universe.Lookup("error").Type()
	)
	return !types.IsInterface(t) && types.AssignableTo(t, errorType)
}

// An isErrorFact records the form a package converts its exported type to
// error in.
type isErrorFact struct {
	Pointer bool   // the pointer form is the one the package writes
	Where   string // where the form was decided, as in "foo.go:12:34"
}

func (*isErrorFact) AFact() {}

func (f *isErrorFact) String() string {
	if f.Pointer {
		return "*E"
	}
	return "E"
}
