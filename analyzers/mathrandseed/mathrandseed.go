// Package mathrandseed provides an analysis.Analyzer that reports a call to
// math/rand.Seed.
//
// The math/rand package documentation says the global generator is seeded
// randomly at program startup when Seed is not called, and marks Seed
// deprecated as of Go 1.20. From Go 1.24 Seed is a no-op by default;
// GODEBUG=randseednop=0 restores its effect
// (https://pkg.go.dev/math/rand#Seed).
//
// For a fixed sequence, call New(NewSource(seed)).
//
// A call inside package math/rand, or in its external test package, is left
// alone, because Seed is declared there and tested by both.
//
// The fix deletes the statement that contains the call. It is withheld when
// the statement is labeled, which would leave the label without a statement;
// when the statement contains a comment; when the call or its argument uses a
// dot import; when the argument is not a constant expression, an identifier,
// or a unary form of one that cannot block or panic; or when a local
// variable's only reads are inside Seed calls.
//
// The fix is offered only in package main of the main module, where a
// //go:debug line is visible. It is withheld when the file's Go version is
// below Go 1.24, when a loaded //go:debug line has a godebug setting that
// restores Seed's effect, and when the driver supplies no version for the
// file. The version is the one the type checker resolved, so a go.work file
// and a go1.x build line are already accounted for. A godebug setting in
// go.mod or in the environment is outside the analysis, because a pass is
// handed neither. The call is still reported, and the report is made at every
// Go version. A //go:debug line in another package, or in a test file the
// driver does not load, is not seen.
//
// The rule is adapted from staticcheck's SA1019.
package mathrandseed

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"go/version"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/source"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "mathrandseed",
	Doc:  "reports a call to math/rand.Seed",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	// The type checker resolved the version in effect for each file, which
	// covers the module's directive, a go.work file, and go1.x build lines.
	byFile := make(map[*ast.File]bool, len(pass.Files))
	for _, f := range pass.Files {
		byFile[f] = pass.Pkg.Name() == "main" && mainModule(pass) && atLeast(
			f, pass.TypesInfo.FileVersions[f],
			"go1.24",
		)
	}
	noGodebug := !packageGodebug(pass.Files)
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			switch n := n.(type) {
			case *ast.ExprStmt:
				call, ok := ast.Unparen(n.X).(*ast.CallExpr)
				if ok && seed(pass, call) {
					report(
						pass, src, file, n, call, stack,
						byFile[file] && noGodebug,
					)
				}
			case *ast.DeferStmt:
				if seed(pass, n.Call) {
					report(
						pass, src, file, n, n.Call,
						stack, byFile[file] && noGodebug,
					)
				}
			case *ast.GoStmt:
				if seed(pass, n.Call) {
					report(
						pass, src, file, n, n.Call,
						stack, byFile[file] && noGodebug,
					)
				}
			}
			return true
		})
	}
	return nil, nil
}

// seed reports whether call calls the package-level math/rand.Seed from
// outside the package that declares it.
func seed(pass *analysis.Pass, call *ast.CallExpr) bool {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil {
		return false
	}
	if declaredHere(pass, fn) {
		return false
	}
	return fn.Pkg().Path() == "math/rand" && fn.Name() == "Seed" &&
		fn.Signature().Recv() == nil
}

// declaredHere reports whether fn belongs to the package under analysis or
// the external test package that tests it.
func declaredHere(pass *analysis.Pass, fn *types.Func) bool {
	owner, subject := packagePath(fn.Pkg()), packagePath(pass.Pkg)
	return owner == subject || owner+"_test" == subject
}

// packagePath returns the path of pkg without the test variant suffix a
// driver adds to a package it builds for testing.
func packagePath(pkg *types.Package) string {
	path, _, _ := strings.Cut(pkg.Path(), " [")
	return path
}

// report reports the call and offers to delete stmt where the fix is safe.
func report(pass *analysis.Pass, src []byte, file *ast.File, stmt ast.Stmt, call *ast.CallExpr, stack []ast.Node, fixable bool) {
	diagnostic := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "math/rand.Seed is deprecated; use New(NewSource(seed)) for a fixed sequence",
	}
	if !canFix(pass, file, stmt, call, stack, fixable) {
		pass.Report(diagnostic)
		return
	}
	diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
		Message: "Delete the statement",
		TextEdits: []analysis.TextEdit{
			removal(pass, src, stmt, keepSemicolon(stack, stmt)),
		},
	}}
	pass.Report(diagnostic)
}

// canFix reports whether the statement can be deleted for call.
func canFix(pass *analysis.Pass, file *ast.File, stmt ast.Stmt, call *ast.CallExpr, stack []ast.Node, fixable bool) bool {
	return fixable && !labeled(stack) && !dotImported(pass, file, call) &&
		droppable(pass, call) && !commented(file, stmt) &&
		!unusedLocal(pass, file, call)
}

// unusedLocal reports whether deleting call leaves a local variable it uses
// with no other use.
func unusedLocal(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) bool {
	var (
		params = parameters(pass, file)
		spans  = seedSpans(pass, file)
		writes = writeOnly(file)
		unused bool
	)
	ast.Inspect(call, func(n ast.Node) bool {
		if unused {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj, ok := pass.TypesInfo.ObjectOf(id).(*types.Var)
		if !ok || !local(pass, obj, params) {
			return true
		}
		if !usedOutside(pass, file, call, obj, spans, writes) {
			unused = true
		}
		return true
	})
	return unused
}

// local reports whether obj is a local variable that is not a parameter.
func local(pass *analysis.Pass, obj *types.Var, params map[types.Object]bool) bool {
	return obj.Pkg() == pass.Pkg && obj.Parent() != pass.Pkg.Scope() &&
		!params[obj]
}

// parameters returns the parameters, results, and receivers declared in file.
func parameters(pass *analysis.Pass, file *ast.File) map[types.Object]bool {
	var (
		out = make(map[types.Object]bool)
		add = func(fields *ast.FieldList) {
			if fields == nil {
				return
			}
			for _, field := range fields.List {
				for _, name := range field.Names {
					if obj := pass.TypesInfo.Defs[name]; obj != nil {
						out[obj] = true
					}
				}
			}
		}
	)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			add(n.Recv)
			add(n.Type.Params)
			add(n.Type.Results)
		case *ast.FuncType:
			add(n.Params)
			add(n.Results)
		}
		return true
	})
	return out
}

// span is a range of the source file.
type span struct {
	pos, end token.Pos
}

// seedSpans returns the spans of the math/rand.Seed calls in file.
func seedSpans(pass *analysis.Pass, file *ast.File) []span {
	var out []span
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && seed(pass, call) {
			out = append(out, span{pos: call.Pos(), end: call.End()})
		}
		return true
	})
	return out
}

// usedOutside reports whether obj has a read in file outside call and outside
// every Seed call, which the fix may also delete.
func usedOutside(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, obj types.Object, spans []span, writes map[*ast.Ident]bool) bool {
	var used bool
	ast.Inspect(file, func(n ast.Node) bool {
		if used {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if pass.TypesInfo.Uses[id] != obj || !outside(call, id) || writes[id] {
			return true
		}
		for _, span := range spans {
			if id.Pos() >= span.pos && id.End() <= span.end {
				return true
			}
		}
		used = true
		return true
	})
	return used
}

// dotImported reports whether call or its argument uses a dot import.
func dotImported(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) bool {
	if _, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		return true
	}
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "." {
			continue
		}
		obj := pass.TypesInfo.Defs[spec.Name]
		if obj == nil {
			obj = pass.TypesInfo.Implicits[spec]
		}
		pkgName, ok := obj.(*types.PkgName)
		if !ok {
			continue
		}
		if packageUsed(pass, call, pkgName.Imported()) {
			return true
		}
	}
	return false
}

// packageUsed reports whether e refers to pkg.
func packageUsed(pass *analysis.Pass, e ast.Node, pkg *types.Package) bool {
	var used bool
	ast.Inspect(e, func(n ast.Node) bool {
		if used {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if obj := pass.TypesInfo.ObjectOf(id); obj != nil && obj.Pkg() == pkg {
			used = true
		}
		return true
	})
	return used
}

// keepSemicolon reports whether stmt is the init of a for loop, where the
// semicolon separates it from the condition.
func keepSemicolon(stack []ast.Node, stmt ast.Stmt) bool {
	if len(stack) < 2 {
		return false
	}
	loop, ok := stack[len(stack)-2].(*ast.ForStmt)
	return ok && loop.Init == stmt
}

// mainModule reports whether the package belongs to the main module, which is
// the module whose go directive sets the godebug default.
func mainModule(pass *analysis.Pass) bool {
	return pass.Module != nil && pass.Module.Main
}

// atLeast reports whether a file's Go version is at least want. A file whose
// version the driver did not supply is treated as older, so no fix is offered
// where the version is unknown.
func atLeast(file *ast.File, v, want string) bool {
	if v == "" {
		return false
	}
	if !strings.HasPrefix(v, "go") {
		v = "go" + v
	}
	return file != nil && version.Compare(v, want) >= 0
}

// outside reports whether id lies outside call.
func outside(call *ast.CallExpr, id *ast.Ident) bool {
	return id.Pos() < call.Pos() || id.End() > call.End()
}

// writeOnly returns the identifiers that an assignment or range target writes
// without reading.
func writeOnly(file *ast.File) map[*ast.Ident]bool {
	out := make(map[*ast.Ident]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok == token.ASSIGN || n.Tok == token.DEFINE {
				for _, lhs := range n.Lhs {
					if id, ok := ast.Unparen(lhs).(*ast.Ident); ok {
						out[id] = true
					}
				}
			}
		case *ast.RangeStmt:
			for _, expr := range []ast.Expr{n.Key, n.Value} {
				if id, ok := ast.Unparen(expr).(*ast.Ident); ok {
					out[id] = true
				}
			}
		}
		return true
	})
	return out
}

// packageGodebug reports whether any file's //go:debug line restores Seed's
// effect.
func packageGodebug(files []*ast.File) bool {
	return slices.ContainsFunc(files, fileGodebug)
}

// fileGodebug reports whether a //go:debug line restores Seed's effect.
func fileGodebug(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, comment := range group.List {
			if unsafeGodebug(debugSetting(comment.Text)) {
				return true
			}
		}
	}
	return false
}

// debugSetting returns the setting of a //go:debug comment, or the empty
// string for any other comment.
func debugSetting(text string) string {
	text = strings.TrimPrefix(text, "//")
	rest, ok := strings.CutPrefix(text, "go:debug")
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return ""
	}
	return strings.TrimLeft(rest, " \t")
}

// unsafeGodebug reports whether one key=value setting restores Seed's effect.
func unsafeGodebug(setting string) bool {
	key, value, ok := strings.Cut(setting, "=")
	if !ok {
		return false
	}
	switch strings.TrimSpace(key) {
	case "randseednop":
		return strings.TrimSpace(value) == "0"
	case "default":
		v := strings.TrimSpace(value)
		if !strings.HasPrefix(v, "go") {
			v = "go" + v
		}
		return version.Compare(v, "go1.24") < 0
	}
	return false
}

// droppable reports whether dropping call leaves no effect and no panic.
func droppable(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	return droppableArg(pass, call.Args[0])
}

// droppableArg reports whether evaluating e can be dropped without an effect,
// a block, or a panic.
func droppableArg(pass *analysis.Pass, e ast.Expr) bool {
	if tv, ok := pass.TypesInfo.Types[e]; ok && tv.Value != nil {
		return true
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.BasicLit, *ast.Ident:
		return true
	case *ast.UnaryExpr:
		return e.Op != token.ARROW && droppableArg(pass, e.X)
	}
	return false
}

// commented reports whether a comment sits inside stmt.
func commented(file *ast.File, stmt ast.Stmt) bool {
	for _, group := range file.Comments {
		if group.Pos() < stmt.End() && group.End() > stmt.Pos() {
			return true
		}
	}
	return false
}

// labeled reports whether a label directly precedes the statement.
func labeled(stack []ast.Node) bool {
	if len(stack) < 2 {
		return false
	}
	_, ok := stack[len(stack)-2].(*ast.LabeledStmt)
	return ok
}

// removal returns the edit that deletes stmt and, unless keep is set, the
// semicolon after it.
func removal(pass *analysis.Pass, src []byte, stmt ast.Stmt, keep bool) analysis.TextEdit {
	var (
		tokFile    = pass.Fset.File(stmt.Pos())
		start, end = tokFile.Offset(stmt.Pos()), tokFile.Offset(stmt.End())
	)
	if keep {
		return analysis.TextEdit{Pos: stmt.Pos(), End: stmt.End()}
	}
	for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
		end++
	}
	if end < len(src) && src[end] == ';' {
		end++
		for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
			end++
		}
	}
	if source.Alone(src, start, end) {
		return analysis.TextEdit{
			Pos: tokFile.Pos(source.Start(src, start)),
			End: tokFile.Pos(source.End(src, end)),
		}
	}
	return analysis.TextEdit{Pos: stmt.Pos(), End: tokFile.Pos(end)}
}
