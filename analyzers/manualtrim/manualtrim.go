// Package manualtrim provides an analysis.Analyzer that reports an if
// statement that trims a string prefix by reslicing it.
//
// The strings.TrimPrefix doc says "If s doesn't start with prefix, s is
// returned unchanged" (https://pkg.go.dev/strings#TrimPrefix). An if statement
// that tests the prefix and then reslices the same string by its length has
// the same effect as an unconditional call.
//
// The condition and the body must use the same value and the same prefix, and
// the body must hold one statement: an assignment of the resliced value back
// to the value the condition tested, written with = and one target and one
// value. A call is not compared, since two calls can return different values.
// The right side must be s[len(prefix):], with the builtin len and no high
// bound. The target must be a name, or a field, index, or dereference rooted
// at a name. An index target whose operand can be a map is left alone, because
// the key would be inserted on the path where the prefix is absent. A field or
// dereference below that index is reported, since the condition already
// dereferences the map value. An if statement with an init clause is left
// alone, because the init and its declarations are lost in the replacement. An
// if statement with an else branch is left alone, because the else branch must
// stay conditional. An else-if gets a block in the replacement, since a bare
// assignment cannot follow else. The replacement contains the package name the
// file uses, or the unqualified TrimPrefix for a dot import; when that name is
// shadowed, the message reports the shadowing and offers no fix. The fix is
// withheld when it would delete a comment.
//
// The rule is inspired by staticcheck's S1017.
package manualtrim

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "manualtrim",
	Doc:  "reports an if statement that trims a string prefix by reslicing it",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		chained := elseIfs(file)
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			if diag, ok := diagnose(pass, file, stmt, chained[stmt]); ok {
				pass.Report(diag)
			}
			return true
		})
	}
	return nil, nil
}

// elseIfs reports the ifs that an else reaches, where the replacement needs a
// block.
func elseIfs(file *ast.File) map[*ast.IfStmt]bool {
	out := make(map[*ast.IfStmt]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		if stmt, ok := n.(*ast.IfStmt); ok {
			if inner, ok := stmt.Else.(*ast.IfStmt); ok {
				out[inner] = true
			}
		}
		return true
	})
	return out
}

// call holds the operands and package name of a strings.HasPrefix call. A nil
// qualifier means a dot import.
type call struct {
	value  ast.Expr
	prefix ast.Expr
	qual   ast.Expr
	named  bool
}

// diagnose returns the report for an if statement that trims a prefix by
// reslicing the string it tested, and its fix where no comment would be
// deleted.
func diagnose(pass *analysis.Pass, file *ast.File, stmt *ast.IfStmt, chained bool) (analysis.Diagnostic, bool) {
	if !trimmedIf(stmt) {
		return analysis.Diagnostic{}, false
	}
	cond, ok := hasPrefix(pass, file, stmt.Cond)
	if !ok {
		return analysis.Diagnostic{}, false
	}
	target, ok := trimmed(pass, stmt.Body.List[0], cond)
	if !ok {
		return analysis.Diagnostic{}, false
	}
	named := cond.named
	fixable := named && !commented(file, stmt.Pos(), stmt.End())
	diag := analysis.Diagnostic{
		Pos:     stmt.Pos(),
		End:     stmt.End(),
		Message: message(cond, named),
	}
	if !fixable {
		return diag, true
	}
	diag.SuggestedFixes = []analysis.SuggestedFix{{
		Message: "Call TrimPrefix",
		TextEdits: []analysis.TextEdit{{
			Pos:     stmt.Pos(),
			End:     stmt.End(),
			NewText: replacement(target, cond, chained),
		}},
	}}
	return diag, true
}

// trimmedIf reports whether stmt has no init clause, no else branch, and one
// statement.
func trimmedIf(stmt *ast.IfStmt) bool {
	return stmt.Init == nil && stmt.Else == nil && len(stmt.Body.List) == 1
}

// hasPrefix returns the operands of cond where it is a strings.HasPrefix call.
func hasPrefix(pass *analysis.Pass, file *ast.File, cond ast.Expr) (call, bool) {
	callExpr, ok := ast.Unparen(cond).(*ast.CallExpr)
	if !ok || len(callExpr.Args) != 2 {
		return call{}, false
	}
	fn := typeutil.StaticCallee(pass.TypesInfo, callExpr)
	if !isHasPrefix(fn) {
		return call{}, false
	}
	qual, named, ok := qualifier(pass, file, callExpr.Fun, callExpr.Pos())
	if !ok {
		return call{}, false
	}
	return call{
		value:  callExpr.Args[0],
		prefix: callExpr.Args[1],
		qual:   qual,
		named:  named,
	}, true
}

// qualifier returns the package name the file uses in front of the call, or
// nil for a dot import, and whether the name is available to the fix.
func qualifier(pass *analysis.Pass, file *ast.File, fun ast.Expr, pos token.Pos) (qual ast.Expr, fixable, ok bool) {
	switch fun := ast.Unparen(fun).(type) {
	case *ast.Ident:
		if shadowed(pass, file, pos) {
			return nil, false, true
		}
		return nil, true, true
	case *ast.SelectorExpr:
		qual := ast.Unparen(fun.X)
		if !packaged(pass, qual) {
			return nil, false, false
		}
		return qual, true, true
	}
	return nil, false, false
}

// shadowed reports whether TrimPrefix at pos resolves to something other than
// the strings.TrimPrefix a dot import adds.
func shadowed(pass *analysis.Pass, file *ast.File, pos token.Pos) bool {
	scope := pass.TypesInfo.Scopes[file]
	if scope == nil {
		return true
	}
	if scope = scope.Innermost(pos); scope == nil {
		return true
	}
	_, obj := scope.LookupParent("TrimPrefix", pos)
	fn, ok := obj.(*types.Func)
	return !ok || fn.Pkg() == nil || fn.Pkg().Path() != "strings"
}

// isHasPrefix reports whether fn is strings.HasPrefix.
func isHasPrefix(fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	return fn.Pkg().Path() == "strings" && fn.Name() == "HasPrefix"
}

// packaged reports whether e names an imported package.
func packaged(pass *analysis.Pass, e ast.Expr) bool {
	_, ok := pass.TypesInfo.Uses[identOf(e)].(*types.PkgName)
	return ok
}

// trimmed returns the value stmt assigns after trimming the prefix, and
// reports false for any other statement.
func trimmed(pass *analysis.Pass, stmt ast.Stmt, cond call) (ast.Expr, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || !sole(assign) {
		return nil, false
	}
	target := ast.Unparen(assign.Lhs[0])
	if !assignable(pass, target) || !astexpr.Same(pass, target, cond.value) {
		return nil, false
	}
	if !resliced(pass, assign.Rhs[0], cond) {
		return nil, false
	}
	return target, true
}

// sole reports whether assign writes one value to one target.
func sole(assign *ast.AssignStmt) bool {
	return assign.Tok == token.ASSIGN &&
		len(assign.Lhs) == 1 && len(assign.Rhs) == 1
}

// assignable reports whether e can be the target of the replacement. An index
// into a map is left alone, since writing it back on the path where the prefix
// is absent would insert the key.
func assignable(pass *analysis.Pass, e ast.Expr) bool {
	return !mapIndex(pass, e) && variable(e)
}

// mapIndex reports whether e is an index into a value whose type can be a map.
func mapIndex(pass *analysis.Pass, e ast.Expr) bool {
	index, ok := ast.Unparen(e).(*ast.IndexExpr)
	if !ok {
		return false
	}
	return mayBeMap(pass.TypesInfo.TypeOf(index.X))
}

// mayBeMap reports whether a value of type t can be a map. A type parameter is
// left to its constraint.
func mayBeMap(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	switch t := t.(type) {
	case *types.Union:
		for term := range t.Terms() {
			if mayBeMap(term.Type()) {
				return true
			}
		}
		return false
	case *types.TypeParam:
		return mayBeMap(t.Constraint())
	}
	switch t := t.Underlying().(type) {
	case *types.Map:
		return true
	case *types.Interface:
		for embedded := range t.EmbeddedTypes() {
			if mayBeMap(embedded) {
				return true
			}
		}
	}
	return false
}

// variable reports whether e is a name, a field path, an index path, or a
// dereference.
func variable(e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return variable(e.X)
	case *ast.IndexExpr:
		return variable(e.X)
	case *ast.StarExpr:
		return variable(e.X)
	}
	return false
}

// resliced reports whether e is value[len(prefix):] for the condition's
// operands.
func resliced(pass *analysis.Pass, e ast.Expr, cond call) bool {
	slice, ok := ast.Unparen(e).(*ast.SliceExpr)
	if !ok || !prefixOnly(slice) {
		return false
	}
	if !astexpr.Same(pass, ast.Unparen(slice.X), cond.value) {
		return false
	}
	return prefixLength(pass, slice.Low, cond.prefix)
}

// prefixOnly reports whether slice has no high bound. The operand is a string,
// so it cannot carry a max index.
func prefixOnly(slice *ast.SliceExpr) bool {
	return slice.High == nil
}

// prefixLength reports whether e is len(prefix) for the prefix the condition
// tests.
func prefixLength(pass *analysis.Pass, e, prefix ast.Expr) bool {
	callExpr, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(callExpr.Args) != 1 {
		return false
	}
	ident, ok := ast.Unparen(callExpr.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[ident] != types.Universe.Lookup("len") {
		return false
	}
	return astexpr.Same(pass, ast.Unparen(callExpr.Args[0]), prefix)
}

// replacement returns the assignment that replaces the if statement. A
// chained if gets a block, since a bare assignment cannot follow else.
func replacement(target ast.Expr, cond call, chained bool) []byte {
	assign := fmt.Sprintf("%s = %s(%s, %s)",
		astexpr.Text(target),
		replacementName(cond),
		astexpr.Text(cond.value),
		astexpr.Text(cond.prefix),
	)
	if chained {
		return []byte("{" + assign + "}")
	}
	return []byte(assign)
}

// replacementName returns the TrimPrefix name the fix writes.
func replacementName(cond call) string {
	if cond.qual == nil {
		return "TrimPrefix"
	}
	return astexpr.Text(cond.qual) + ".TrimPrefix"
}

// message returns the diagnostic for cond, with the TrimPrefix name when the
// file has a name for it.
func message(cond call, named bool) string {
	if !named {
		return "manual prefix trim; a TrimPrefix name shadows the import"
	}
	return "manual prefix trim; use " + replacementName(cond)
}

// identOf returns the identifier e is, or nil when e is not one.
func identOf(e ast.Expr) *ast.Ident {
	ident, _ := e.(*ast.Ident)
	return ident
}

// commented reports whether a comment sits in the span a fix would replace.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
