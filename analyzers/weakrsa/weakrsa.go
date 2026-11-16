// Package weakrsa provides an analysis.Analyzer that reports a call to
// rsa.GenerateKey with a constant key size below 2048 bits.
//
// The crypto/rsa package documentation has a "Minimum key size" section:
// rsa.GenerateKey returns an error for a key of less than 1024 bits, and such
// keys "are insecure and should not be used"
// (https://pkg.go.dev/crypto/rsa#hdr-Minimum_key_size). The rsa1024min=0
// GODEBUG setting suppresses that error. NIST SP 800-57 Part 1 Rev. 5
// recommends at least 2048 through 2030
// (https://doi.org/10.6028/NIST.SP.800-57pt1r5).
//
// By default, Go accepts a 1024-bit key, and the rule reports it for that
// recommendation.
//
// A constant key size below 2048 is reported, including 0 and negative
// values. A literal, a named constant, a conversion of a constant, and an
// expression such as 1024/2 are all constant. A bare variable, a conversion of
// a variable, and a call with a non-constant result are left alone, because
// they are not constant expressions. A constant expression that uses a local
// variable, as len(buf) does, is reported. A call through a function value is
// left alone too, and so is a call in a _test.go file, where a test uses a
// small key.
//
// The fix raises the key size to 2048 bits by replacing the argument with the
// decimal 2048. It is withheld for a constant below 1024, because the call
// returns an error and the fix would remove that error path. It is withheld
// when a comment overlaps the argument, because the replacement would remove
// it. It is withheld when the argument uses a named constant, because the
// declaration would still be 1024. It is withheld when the argument uses a
// local variable, a name a dot import brings in, or a package-qualified name,
// because the replacement could leave the local variable or the import unused.
//
// A deliberate 1024-bit key is kept with an //ignore:weakrsa directive on the
// line with the key size.
//
// The rule is adapted from gosec's G403, which reports a key below 2048 bits.
package weakrsa

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

// limit is the recommended key size.
const limit = 2048

// minimum is the size below which rsa.GenerateKey returns an error.
const minimum = 1024

var Analyzer = &analysis.Analyzer{
	Name: "weakrsa",
	Doc:  "reports rsa.GenerateKey with a constant key size below 2048 bits",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 || !generates(pass, call) {
				return true
			}
			arg := ast.Unparen(call.Args[1])
			if bits, ok := size(pass, arg); ok {
				report(pass, file, arg, bits)
			}
			return true
		})
	}
	return nil, nil
}

// generates reports whether call resolves to the package-level
// rsa.GenerateKey.
func generates(pass *analysis.Pass, call *ast.CallExpr) bool {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == "crypto/rsa" &&
		fn.Name() == "GenerateKey" && fn.Signature().Recv() == nil
}

// size returns the argument's constant value when it is below limit.
func size(pass *analysis.Pass, arg ast.Expr) (int64, bool) {
	tv, ok := pass.TypesInfo.Types[arg]
	if !ok || tv.Value == nil {
		return 0, false
	}
	v := constant.ToInt(tv.Value)
	if v.Kind() != constant.Int {
		return 0, false
	}
	if !constant.Compare(v, token.LSS, constant.MakeInt64(limit)) {
		return 0, false
	}
	bits, ok := constant.Int64Val(v)
	return bits, ok
}

// report adds a diagnostic at arg and, when the fix is safe to offer, a fix
// that raises the size to limit.
func report(pass *analysis.Pass, file *ast.File, arg ast.Expr, bits int64) {
	var (
		diag = analysis.Diagnostic{
			Pos: arg.Pos(),
			End: arg.End(),
			Message: "RSA key size " + strconv.FormatInt(bits, 10) +
				" is below the recommended " + strconv.Itoa(limit) + " bits",
		}
		replacement = strconv.Itoa(limit)
	)
	if fixable(pass, file, arg, bits) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Raise the key size to " + replacement,
			TextEdits: []analysis.TextEdit{{
				Pos:     arg.Pos(),
				End:     arg.End(),
				NewText: []byte(replacement),
			}},
		}}
	}
	pass.Report(diag)
}

// fixable reports whether the fix for arg is safe to offer.
func fixable(pass *analysis.Pass, file *ast.File, arg ast.Expr, bits int64) bool {
	if bits < minimum {
		return false
	}
	return !commented(file, arg.Pos(), arg.End()) &&
		!dotImported(pass, file, arg) &&
		!namedConstant(pass, arg) &&
		!qualifiedName(pass, arg) &&
		!localName(pass, arg)
}

// namedConstant reports whether expr uses a named constant, whose declaration
// would still hold the old size.
func namedConstant(pass *analysis.Pass, expr ast.Expr) bool {
	var named bool
	ast.Inspect(expr, func(n ast.Node) bool {
		if named {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := pass.TypesInfo.Uses[id].(*types.Const); ok {
			named = true
			return false
		}
		return true
	})
	return named
}

// localName reports whether arg uses a local variable, which the fix could
// leave unused.
func localName(pass *analysis.Pass, arg ast.Expr) bool {
	var local bool
	ast.Inspect(arg, func(n ast.Node) bool {
		if local {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := pass.TypesInfo.Uses[id]
		if obj == nil || obj.Pkg() != pass.Pkg {
			return true
		}
		v, ok := obj.(*types.Var)
		if !ok || v.IsField() {
			return true
		}
		if v.Parent() != pass.Pkg.Scope() {
			local = true
			return false
		}
		return true
	})
	return local
}

// commented reports whether a comment overlaps [from, to).
func commented(file *ast.File, from, to token.Pos) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment.Pos() < to && comment.End() > from {
				return true
			}
		}
	}
	return false
}

// qualifiedName reports whether expr uses a package-qualified name whose
// import the fix could leave unused.
func qualifiedName(pass *analysis.Pass, expr ast.Expr) bool {
	var qualified bool
	ast.Inspect(expr, func(n ast.Node) bool {
		if qualified {
			return false
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := ast.Unparen(sel.X).(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := pass.TypesInfo.Uses[id].(*types.PkgName); ok {
			qualified = true
			return false
		}
		return true
	})
	return qualified
}

// dotImported reports whether expr uses a name a dot import brings in, whose
// import the fix could leave unused.
func dotImported(pass *analysis.Pass, file *ast.File, expr ast.Expr) bool {
	dot := make(map[string]struct{})
	for _, imp := range file.Imports {
		if imp.Name == nil || imp.Name.Name != "." {
			continue
		}
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		for _, pkg := range pass.Pkg.Imports() {
			if pkg.Path() == path {
				dot[path] = struct{}{}
				break
			}
		}
	}
	if len(dot) == 0 {
		return false
	}
	var used bool
	ast.Inspect(expr, func(n ast.Node) bool {
		if used {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := pass.TypesInfo.Uses[id]
		if obj == nil || obj.Pkg() == nil {
			return true
		}
		if _, ok := dot[obj.Pkg().Path()]; ok {
			used = true
			return false
		}
		return true
	})
	return used
}
