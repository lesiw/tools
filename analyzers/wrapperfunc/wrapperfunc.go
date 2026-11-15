// Package wrapperfunc provides an analysis.Analyzer that reports a declared
// function that only forwards its arguments.
//
// A function that forwards its own parameters to another function of the same
// signature adds a second name for behavior that already has one, and a caller
// could name the other function instead.
//
// Report only: the callers would each have to be rewritten, and a caller's
// file might not already import the callee. The README catalogs every check,
// exemption, and limit.
package wrapperfunc

import (
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "wrapperfunc",
	Doc:  "reports a declared function that only forwards its arguments",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	c := &checker{
		pass:      pass,
		held:      uses.Held(pass),
		methods:   uses.InterfaceMethods(pass),
		published: published(pass),
	}
	for _, file := range pass.Files {
		name := pass.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(name, "_test.go") || constrained(name, file) {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if callee, ok := c.reportable(file, fn); ok {
				word := "func"
				if fn.Recv != nil {
					word = "method"
				}
				pass.Reportf(
					fn.Name.Pos(),
					"%s %s only forwards its arguments; call %s instead",
					word, fn.Name.Name,
					types.ExprString(ast.Unparen(callee)),
				)
			}
		}
	}
	return nil, nil
}

// checker is one package's files, scanned once for the facts every candidate
// needs: the functions held as values, the methods its interfaces declare, and
// the names a directive publishes to the linker.
type checker struct {
	pass      *analysis.Pass
	held      map[*types.Func]struct{}
	methods   uses.Declarations
	published map[string]struct{}
}

// reportable returns the callee of the call fn makes, when fn only forwards
// its arguments to a function a caller could call instead.
func (c *checker) reportable(file *ast.File, fn *ast.FuncDecl) (ast.Expr, bool) {
	self, ok := c.pass.TypesInfo.Defs[fn.Name].(*types.Func)
	if !ok {
		return nil, false
	}
	if self.Exported() {
		return nil, false
	}
	if entrypoint(self) {
		return nil, false
	}
	if c.documented(file, fn) {
		return nil, false
	}
	if _, isHeld := c.held[self]; isHeld {
		return nil, false
	}
	if c.methods.Has(self) {
		return nil, false
	}
	return c.forwards(fn, self)
}

// forwards returns the callee of the call fn makes, when it forwards fn's own
// parameters, in order, to a function of fn's own signature.
func (c *checker) forwards(fn *ast.FuncDecl, self *types.Func) (ast.Expr, bool) {
	call := soleCall(fn.Body)
	if call == nil {
		return nil, false
	}
	_, callee, ok := uses.CalleeOf(c.pass, call.Fun)
	if !ok || callee == self {
		return nil, false
	}
	if methodValue(c.pass, call.Fun) {
		return nil, false
	}
	selfSig := self.Signature()
	// The call site's signature is the callee's, instantiated when the callee
	// is generic, so it is the one type here the declaration does not carry.
	callSig, ok := c.pass.TypesInfo.TypeOf(call.Fun).(*types.Signature)
	if !ok {
		return nil, false
	}
	if !uses.SameSignature(selfSig, callSig) {
		return nil, false
	}
	if !c.forwardsArgs(call, selfSig) {
		return nil, false
	}
	return call.Fun, true
}

// methodValue reports whether expr names a method value, whose receiver the
// value carries. A method expression names a function that takes the receiver
// as its first argument, which the wrapper may pass through like any other.
func methodValue(pass *analysis.Pass, expr ast.Expr) bool {
	selector, ok := ast.Unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok || fn.Signature().Recv() == nil {
		return false
	}
	return !pass.TypesInfo.Types[selector.X].IsType()
}

// soleCall returns the call that body makes, when body is a single call or a
// single return of one call.
func soleCall(body *ast.BlockStmt) *ast.CallExpr {
	if len(body.List) != 1 {
		return nil
	}
	var expr ast.Expr
	switch stmt := body.List[0].(type) {
	case *ast.ExprStmt:
		expr = stmt.X
	case *ast.ReturnStmt:
		if len(stmt.Results) != 1 {
			return nil
		}
		expr = stmt.Results[0]
	default:
		return nil
	}
	call, _ := ast.Unparen(expr).(*ast.CallExpr)
	return call
}

// entrypoint reports whether fn is init or a program's main, which nothing
// calls.
func entrypoint(fn *types.Func) bool {
	if fn.Signature().Recv() != nil {
		return false
	}
	if fn.Name() == "init" {
		return true
	}
	return fn.Name() == "main" && fn.Pkg().Name() == "main"
}

// forwardsArgs reports whether call passes the parameters of sig through in
// order, unchanged.
func (c *checker) forwardsArgs(call *ast.CallExpr, sig *types.Signature) bool {
	if len(call.Args) != sig.Params().Len() {
		return false
	}
	for i, arg := range call.Args {
		id, ok := ast.Unparen(arg).(*ast.Ident)
		if !ok || c.pass.TypesInfo.Uses[id] != sig.Params().At(i) {
			return false
		}
	}
	return true
}

// constrained reports whether another configuration may run a different file
// in this one's place, by a constraint line or a platform suffix on the name.
func constrained(path string, file *ast.File) bool {
	base := strings.TrimSuffix(filepath.Base(path), ".go")
	if strings.HasSuffix(base, "_"+build.Default.GOOS) {
		return true
	}
	if strings.HasSuffix(base, "_"+build.Default.GOARCH) {
		return true
	}
	for _, group := range file.Comments {
		if group.Pos() > file.Package {
			break
		}
		for _, c := range group.List {
			if constraint.IsGoBuild(c.Text) || constraint.IsPlusBuild(c.Text) {
				return true
			}
		}
	}
	return false
}

// documented reports whether a comment says what the wrapper is for. A
// directive is an instruction rather than an explanation, so it does not
// count, except one that publishes the name outside the package, which has to
// stay.
func (c *checker) documented(file *ast.File, fn *ast.FuncDecl) bool {
	if _, ok := c.published[fn.Name.Name]; ok {
		return true
	}
	if attached(fn) {
		return true
	}
	if fn.Doc.Text() != "" {
		return true
	}
	for _, group := range file.Comments {
		if group.Pos() <= fn.Body.Lbrace {
			continue
		}
		if group.End() >= fn.Body.Rbrace {
			continue
		}
		if group.Text() != "" {
			return true
		}
	}
	return false
}

// published returns the names a directive in the package publishes to the
// linker. The directive applies by local name wherever it is written, so the
// whole package is scanned.
func published(pass *analysis.Pass) map[string]struct{} {
	out := make(map[string]struct{})
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			for _, c := range group.List {
				if name, ok := linknamed(c.Text); ok {
					out[name] = struct{}{}
				}
			}
		}
	}
	return out
}

// linknamed returns the local name a directive publishes to the linker.
func linknamed(text string) (string, bool) {
	rest, ok := strings.CutPrefix(text, "//go:linkname ")
	if !ok {
		rest, ok = strings.CutPrefix(text, "//go:linknamestd ")
	}
	if !ok {
		return "", false
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", false
	}
	return fields[0], true
}

// attached reports whether a directive on fn publishes it to something outside
// the package that names it by attachment: a wasm export, or cgo's //export.
func attached(fn *ast.FuncDecl) bool {
	if fn.Doc == nil {
		return false
	}
	for _, c := range fn.Doc.List {
		if strings.HasPrefix(c.Text, "//go:wasmexport ") {
			return true
		}
		if strings.HasPrefix(c.Text, "//export ") {
			return true
		}
	}
	return false
}
