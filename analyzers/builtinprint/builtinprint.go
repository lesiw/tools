// Package builtinprint provides an analysis.Analyzer that reports calls to the
// predeclared print and println functions.
//
// The builtin package documents both as bootstrapping aids: they format their
// arguments in an implementation-specific way, they write to standard error,
// and they are "not guaranteed to stay in the language".
//
// The fix writes the call with fmt and keeps the output on standard error. It
// is withheld where fmt would write something else, and where the file cannot
// take the rewrite. A missing import is added as a declaration of its own, so
// a driver that only formats the file still writes code that compiles. The
// rule reports only in packages that belong to a module. The README lists the
// cases.
//
// Like revive's use-fmt-print.
package builtinprint

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "builtinprint",
	Doc:  "reports calls to print or println",
	Run:  run,
}

// replace describes how fmt writes a predeclared function's call.
var replace = map[string]struct {
	call      string // the fmt function that takes the builtin's place
	separated bool   // fmt separates two non-string operands, as println does
}{
	"print":   {call: "Fprint"},
	"println": {call: "Fprintln", separated: true},
}

func run(pass *analysis.Pass) (any, error) {
	// A driver reports no module, or an empty one, for the standard library,
	// the compiler, and a tree built without modules.
	if pass.Module == nil || pass.Module.Path == "" {
		return nil, nil
	}
	for _, file := range pass.Files {
		names := imported(pass, file)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := callee(call)
			if !ok {
				return true
			}
			if _, ok := replace[ident.Name]; !ok {
				return true
			}
			// A declaration of the same name shadows the builtin, and the
			// type checker has already resolved which one the call names.
			obj := types.Universe.Lookup(ident.Name)
			if pass.TypesInfo.Uses[ident] != obj {
				return true
			}
			pass.Report(diagnose(pass, file, names, call, ident.Name))
			return true
		})
	}
	return nil, nil
}

// callee returns the identifier call names, through parentheses.
func callee(call *ast.CallExpr) (*ast.Ident, bool) {
	fun := call.Fun
	for {
		paren, ok := fun.(*ast.ParenExpr)
		if !ok {
			break
		}
		fun = paren.X
	}
	ident, ok := fun.(*ast.Ident)
	return ident, ok
}

// diagnose returns the diagnostic for one call, with the rewrite as a fix when
// the rewrite can be written.
func diagnose(pass *analysis.Pass, file *ast.File, names map[string]string, call *ast.CallExpr, builtin string) analysis.Diagnostic {
	var (
		fmtCall        = replace[builtin].call
		fmtName, fmtOK = qualifier(pass, names, "fmt", call.Pos())
		osName, osOK   = qualifier(pass, names, "os", call.Pos())
		prefix         = fmtName + "." + fmtCall + "(" + osName + ".Stderr"
		replacement    = prefix + ", ...)"
		diagnostic     = analysis.Diagnostic{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: builtin + " is not guaranteed to stay in the language",
		}
	)
	if !fmtOK || !osOK || !sameOutput(pass, call, builtin) {
		return diagnostic
	}
	diagnostic.Message = builtin + " can be replaced by " + replacement
	args, ok := arguments(pass, call)
	if !ok {
		return diagnostic
	}
	edits := []analysis.TextEdit{{
		Pos:     call.Pos(),
		End:     call.End(),
		NewText: []byte(prefix + args + ")"),
	}}
	if missing := absent(names, "fmt", "os"); len(missing) > 0 {
		edits = append(edits, addImports(pass, file, missing))
	}
	diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
		Message:   "Replace " + builtin + " with " + replacement,
		TextEdits: edits,
	}}
	return diagnostic
}

// sameOutput reports whether fmt writes call's operands the way the builtin
// writes them.
func sameOutput(pass *analysis.Pass, call *ast.CallExpr, builtin string) bool {
	var (
		typ  = pass.TypesInfo.TypeOf
		sep  = replace[builtin].separated
		prev types.Type
	)
	for _, arg := range call.Args {
		cur := typ(arg)
		// The builtin prints the value itself. fmt calls a method named Error,
		// Format, or String on it first, prints the value a pointer points to
		// rather than its address, and prints a slice's elements rather than
		// its length, capacity, and address. gc's print routines write a
		// boolean, a number, and a string the way fmt does.
		if basic(cur)&types.IsConstType == 0 || hasFormatMethod(cur) {
			return false
		}
		// print runs its operands together. Fprint separates two operands
		// when neither is a string, so that pair gains a space.
		if prev != nil && !sep && !isString(prev) && !isString(cur) {
			return false
		}
		prev = cur
	}
	return true
}

// hasFormatMethod reports whether t has a method fmt may call instead of
// printing the value. The method is matched by name, because the signatures
// that decide live in the fmt package.
func hasFormatMethod(t types.Type) bool {
	methods := types.NewMethodSet(t)
	for _, name := range []string{"Error", "Format", "String"} {
		if methods.Lookup(nil, name) != nil {
			return true
		}
	}
	return false
}

// basic returns what t's basic type says about it, or 0 when t is not a basic
// type.
func basic(t types.Type) types.BasicInfo {
	if t == nil {
		return 0
	}
	if b, ok := t.Underlying().(*types.Basic); ok {
		return b.Info()
	}
	return 0
}

// isString reports whether t is a string, which Fprint does not separate from
// its neighbor.
func isString(t types.Type) bool {
	return basic(t)&types.IsString != 0
}

// imported maps the path of each import in file to the name the file binds it
// to.
func imported(pass *analysis.Pass, file *ast.File) map[string]string {
	names := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		obj := pass.TypesInfo.Defs[spec.Name]
		if obj == nil {
			obj = pass.TypesInfo.Implicits[spec]
		}
		if pkgName, ok := obj.(*types.PkgName); ok {
			names[pkgName.Imported().Path()] = pkgName.Name()
		}
	}
	return names
}

// qualifier returns the name the rewrite writes for the package imported at
// importPath, and reports whether the name reaches that package at pos.
func qualifier(pass *analysis.Pass, names map[string]string, importPath string, pos token.Pos) (string, bool) {
	name, ok := names[importPath]
	if ok && (name == "." || name == "_") {
		return path.Base(importPath), false
	}
	if !ok {
		name = path.Base(importPath)
	}
	// A declaration or another import can hold the name the rewrite writes.
	if scope := pass.Pkg.Scope().Innermost(pos); scope != nil {
		if _, obj := scope.LookupParent(name, pos); obj != nil {
			pkgName, ok := obj.(*types.PkgName)
			if !ok || pkgName.Imported().Path() != importPath {
				return name, false
			}
		}
	}
	return name, true
}

// arguments returns call's arguments as the tail of an argument list, in the
// source text as written, so a rewrite keeps the line breaks and comments
// among them.
func arguments(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	tokFile := pass.Fset.File(call.Pos())
	if tokFile == nil {
		return "", false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return "", false
	}
	open := pass.Fset.Position(call.Lparen).Offset + 1
	close := pass.Fset.Position(call.Rparen).Offset
	if open >= close {
		return "", true
	}
	return ", " + string(src[open:close]), true
}

// absent returns the paths that the file has no import for.
func absent(names map[string]string, importPaths ...string) (missing []string) {
	for _, importPath := range importPaths {
		if _, ok := names[importPath]; !ok {
			missing = append(missing, importPath)
		}
	}
	return missing
}

// addImports returns the edit that adds importPaths as a declaration after the
// package clause, which a driver that only formats leaves in place and
// goimports folds into the file's own block.
func addImports(pass *analysis.Pass, file *ast.File, importPaths []string) analysis.TextEdit {
	var (
		tokFile = pass.Fset.File(file.Pos())
		b       strings.Builder
	)
	b.WriteString("\n\nimport (\n")
	for _, importPath := range importPaths {
		b.WriteString("\t" + strconv.Quote(importPath) + "\n")
	}
	b.WriteString(")")
	pos := tokFile.LineStart(tokFile.Line(file.Name.Pos())+1) - 1
	return analysis.TextEdit{Pos: pos, End: pos, NewText: []byte(b.String())}
}
