// Package wrapperarg provides an analysis.Analyzer that reports a call that
// passes an argument a convenience wrapper already supplies.
//
// Each entry comes from Go's docs. sync.WaitGroup.Done
// "is equivalent to Add(-1)", bytes.Buffer.Reset "is the same as
// Buffer.Truncate(0)", and image/draw.Draw "calls DrawMask with a nil mask".
// strings.Split is "equivalent to SplitN with a count of -1", and
// strings.SplitAfter is "equivalent to SplitAfterN with a count of -1".
// bytes.Split and bytes.SplitAfter say the same.
//
// The mapping entries use a pair of docs. strings.ToUpper,
// strings.ToLower, and strings.ToTitle map every letter to one case, and
// unicode.ToUpper, unicode.ToLower, and unicode.ToTitle are the mappings that
// do it. A strings.Map call with one of those mappings produces the same
// result one rune at a time. The bytes functions pair the same way.
//
// bytes.Replace with a count of -1 is bytes.ReplaceAll. Replace's doc gives a
// negative count "no limit on the number of replacements", and ReplaceAll's
// doc replaces every non-overlapping instance. strings.Replace takes the same
// shape, and strreplaceall reports it.
//
// The rule matches the fixed argument as it is written at the call: a negative
// integer literal for a count, the literal -1 for a delta, an integer literal
// with the value zero, nil, or one of the three unicode case mappings. A count
// held in a constant or a variable, and a mapping held in a variable, are left
// alone.
//
// Every argument the fix deletes has to be evaluable without running code
// or panicking, since deleting it removes that work. DrawMask's mask point is
// the argument this matters for: a call there is reported without a fix.
//
// The call has to name the function through a selector. A call through a dot
// import is left alone, because a name in scope could shadow the target.
// A call through a function value is left alone, since there is no selector
// to rename. A method expression passes the receiver as its first argument, so
// the table's argument indexes do not apply. A call on an interface is left
// alone too, since the implementation is not visible here. A call is left
// alone when the receiver's type embeds one of these and declares a method
// with the target's name, since the rewrite would call that method. A call
// inside the target's declaration is left alone too, since the rewrite would
// call the function from its body. sync.WaitGroup.Done is such a call: its
// body is Add(-1).
//
// The fix renames the function and deletes the fixed arguments. It is withheld
// where a comment sits in the text it would delete, where deleting an argument
// would leave a local variable declared and not used, and where an argument
// names something from a dot-imported package; the call is still reported.
//
// The rule is adapted from go-critic's wrapperFunc.
//
// The README catalogs every check, exemption, and fix.
package wrapperarg

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "wrapperarg",
	Doc:  "reports a call that passes an argument a wrapper already supplies",
	Run:  run,
}

// wrapper is one table entry: a call and the function that takes the fixed
// argument out of its signature.
type wrapper struct {
	// call is the full name of the function the call names, as
	// (*types.Func).FullName reports it.
	call string

	// want reports whether the fixed argument holds the value the target
	// supplies.
	want func(*analysis.Pass, ast.Expr) bool

	// target is the name of the function to call instead.
	target string

	// drop lists the arguments the target does not take, in order, and the
	// indexes are consecutive. The first one holds the value want reads.
	drop []int

	// note names the fixed argument in the diagnostic. An empty note uses
	// the argument's source text.
	note string
}

// wrappers is the table. Each entry names a call and the function that leaves
// one of its arguments out.
var wrappers = []wrapper{{
	call:   "strings.SplitN",
	want:   negative,
	target: "Split",
	drop:   []int{2},
	note:   "a negative count",
}, {
	call:   "strings.SplitAfterN",
	want:   negative,
	target: "SplitAfter",
	drop:   []int{2},
	note:   "a negative count",
}, {
	call:   "bytes.SplitN",
	want:   negative,
	target: "Split",
	drop:   []int{2},
	note:   "a negative count",
}, {
	call:   "bytes.SplitAfterN",
	want:   negative,
	target: "SplitAfter",
	drop:   []int{2},
	note:   "a negative count",
}, {
	call:   "bytes.Replace",
	want:   negative,
	target: "ReplaceAll",
	drop:   []int{3},
	note:   "a negative count",
}, {
	call:   "strings.Map",
	want:   mapping("ToUpper"),
	target: "ToUpper",
	drop:   []int{0},
}, {
	call:   "strings.Map",
	want:   mapping("ToLower"),
	target: "ToLower",
	drop:   []int{0},
}, {
	call:   "strings.Map",
	want:   mapping("ToTitle"),
	target: "ToTitle",
	drop:   []int{0},
}, {
	call:   "bytes.Map",
	want:   mapping("ToUpper"),
	target: "ToUpper",
	drop:   []int{0},
}, {
	call:   "bytes.Map",
	want:   mapping("ToLower"),
	target: "ToLower",
	drop:   []int{0},
}, {
	call:   "bytes.Map",
	want:   mapping("ToTitle"),
	target: "ToTitle",
	drop:   []int{0},
}, {
	call:   "(*sync.WaitGroup).Add",
	want:   minusOne,
	target: "Done",
	drop:   []int{0},
	note:   "a delta of -1",
}, {
	call:   "(*bytes.Buffer).Truncate",
	want:   zero,
	target: "Reset",
	drop:   []int{0},
	note:   "a length of 0",
}, {
	call:   "image/draw.DrawMask",
	want:   nothing,
	target: "Draw",
	drop:   []int{4, 5},
	note:   "a nil mask",
}}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, call)
			}
			return true
		})
	}
	return nil, nil
}

// match is one call the rule reports: the selector it names, the function,
// the table entry, and the function to call instead.
type match struct {
	sel    *ast.SelectorExpr
	fn     *types.Func
	target *types.Func
	w      wrapper
}

// check reports call when one of the wrappers applies to it.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) {
	m, ok := matchOf(pass, file, call)
	if !ok {
		return
	}
	report(pass, file, call, m)
}

// matchOf returns the match for call, when the rule reports on it.
func matchOf(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) (match, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return match{}, false
	}
	// A method expression passes the receiver as its first argument, so the
	// argument indexes in the table would be off by one.
	s := pass.TypesInfo.Selections[sel]
	if s != nil && s.Kind() != types.MethodVal {
		return match{}, false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok {
		return match{}, false
	}
	for _, w := range wrappers {
		if fn.FullName() != w.call || w.drop[len(w.drop)-1] >= len(call.Args) {
			continue
		}
		if !w.want(pass, call.Args[w.drop[0]]) {
			continue
		}
		target, ok := targetOf(pass, sel, fn, w.target)
		if !ok {
			continue
		}
		if insideTarget(pass, file, call, target) {
			continue
		}
		return match{sel: sel, fn: fn, target: target, w: w}, true
	}
	return match{}, false
}

// targetOf returns the function a wrapper's target names, when the call's
// receiver reaches that function. A type that embeds the wrapper's type and
// declares a method of the target's name reaches that method, and the
// rewrite would call that one.
func targetOf(pass *analysis.Pass, sel *ast.SelectorExpr, fn *types.Func, target string) (*types.Func, bool) {
	var (
		expected = strings.TrimSuffix(fn.FullName(), fn.Name()) + target
		obj      types.Object
	)
	if fn.Signature().Recv() != nil {
		obj, _, _ = types.LookupFieldOrMethod(
			pass.TypesInfo.TypeOf(sel.X),
			true, pass.Pkg, target,
		)
	} else {
		obj = fn.Pkg().Scope().Lookup(target)
	}
	t, ok := obj.(*types.Func)
	if !ok || t.FullName() != expected {
		return nil, false
	}
	return t, true
}

// droppable reports whether every argument the fix deletes can be evaluated
// without running code or panicking, since deleting it removes that work.
func droppable(pass *analysis.Pass, call *ast.CallExpr, drop []int) bool {
	for _, i := range drop {
		if !astexpr.Safe(pass, call.Args[i]) {
			return false
		}
	}
	return true
}

// dotImported reports whether the deleted arguments name something from a
// package the file dot-imports, since the edit could leave that import unused.
func dotImported(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, drop []int) bool {
	var paths []string
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "." {
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return false
	}
	for _, i := range drop {
		var found bool
		ast.Inspect(call.Args[i], func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			obj := pass.TypesInfo.Uses[id]
			if obj == nil || obj.Pkg() == nil {
				return true
			}
			for _, path := range paths {
				if obj.Pkg().Path() == path {
					found = true
					return false
				}
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// insideTarget reports whether call sits inside the declaration of fn. A call
// in the target's body can become a recursive call after the rewrite, as in
// sync.WaitGroup.Done, whose body is wg.Add(-1). The check does not read the
// receiver, so it also skips a call on another value in that body.
func insideTarget(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, fn *types.Func) bool {
	path, _ := astutil.PathEnclosingInterval(file, call.Pos(), call.End())
	for _, node := range path {
		decl, ok := node.(*ast.FuncDecl)
		if !ok {
			continue
		}
		enclosing, ok := pass.TypesInfo.Defs[decl.Name].(*types.Func)
		return ok && enclosing.FullName() == fn.FullName()
	}
	return false
}

// report adds the diagnostic for one wrapper, and its fix where the edit is
// safe.
func report(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, m match) {
	var (
		callName, targetName = names(pass, m.sel, m.fn, m.target)
		note                 = m.w.note
		drop                 = m.w.drop
		diag                 = analysis.Diagnostic{
			Pos: call.Pos(),
			End: call.End(),
		}
		fix = analysis.SuggestedFix{
			Message: "Replace " + callName + " with " + targetName,
		}
	)
	if note == "" {
		note = astexpr.Text(call.Args[drop[0]])
	}
	if !droppable(pass, call, drop) {
		diag.Message = callName + " with " + note + " evaluates an argument " +
			targetName + " would not evaluate"
		pass.Report(diag)
		return
	}
	diag.Message = callName + " with " + note +
		" can be replaced by " + targetName
	fix.TextEdits = append(fix.TextEdits, analysis.TextEdit{
		Pos:     m.sel.Sel.Pos(),
		End:     m.sel.Sel.End(),
		NewText: []byte(m.target.Name()),
	})
	start, end := span(call, drop)
	if loses(pass, file, call, drop) || orphaned(pass, file, call, drop) {
		pass.Report(diag)
		return
	}
	fix.TextEdits = append(fix.TextEdits, analysis.TextEdit{
		Pos: start,
		End: end,
	})
	diag.SuggestedFixes = []analysis.SuggestedFix{fix}
	pass.Report(diag)
}

// span returns the range of a call that holds the arguments drop names. The
// range takes in one adjoining comma, so the argument list stays well formed.
func span(call *ast.CallExpr, drop []int) (start, end token.Pos) {
	first, last := drop[0], drop[len(drop)-1]
	switch {
	case last+1 < len(call.Args):
		return call.Args[first].Pos(), call.Args[last+1].Pos()
	case first > 0:
		return call.Args[first-1].End(), call.Args[last].End()
	default:
		// Every argument goes, so the range covers the parentheses'
		// contents, including a trailing comma.
		return call.Lparen + 1, call.Rparen
	}
}

// commented reports whether a comment sits in [start, end).
func commented(file *ast.File, start, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > start {
			return true
		}
	}
	return false
}

// loses reports whether the edit would delete a comment or the last naming of
// a dot-imported package in file.
func loses(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, drop []int) bool {
	start, end := span(call, drop)
	return commented(file, start, end) || dotImported(pass, file, call, drop)
}

// orphaned reports whether deleting the arguments would leave a local variable
// declared and not used, which Go rejects. A parameter, a result, a field, and
// a package-level variable may stay unused.
func orphaned(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, drop []int) bool {
	var (
		path    []ast.Node
		written = writes(file)
		gone    = dropped(pass, file)
		vars    = make(map[*types.Var]struct{})
	)
	path, _ = astutil.PathEnclosingInterval(file, call.Pos(), call.End())
	for _, i := range drop {
		ast.Inspect(call.Args[i], func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				if v, ok := pass.TypesInfo.Uses[id].(*types.Var); ok {
					vars[v] = struct{}{}
				}
			}
			return true
		})
	}
	for v := range vars {
		if v.IsField() || !local(pass, path, v) {
			continue
		}
		if !read(pass, written, gone, v) {
			return true
		}
	}
	return false
}

// dropped returns the identifiers inside the arguments the fix can delete in
// file, over every call the rule matches.
func dropped(pass *analysis.Pass, file *ast.File) map[*ast.Ident]struct{} {
	out := make(map[*ast.Ident]struct{})
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		m, ok := matchOf(pass, file, call)
		if !ok || !droppable(pass, call, m.w.drop) {
			return true
		}
		if loses(pass, file, call, m.w.drop) {
			return true
		}
		for _, i := range m.w.drop {
			ast.Inspect(call.Args[i], func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					out[id] = struct{}{}
				}
				return true
			})
		}
		return true
	})
	return out
}

// local reports whether v is a variable declared in a function body, where
// an unused declaration does not compile. A parameter, a result, a receiver,
// and a package-level variable may stay unused.
func local(pass *analysis.Pass, path []ast.Node, v *types.Var) bool {
	if v.Pkg() != pass.Pkg || v.Parent() == pass.Pkg.Scope() {
		return false
	}
	for _, node := range path {
		switch node := node.(type) {
		case *ast.FuncDecl:
			t := node.Type
			if declares(pass, v, node.Recv, t.Params, t.Results) {
				return false
			}
		case *ast.FuncLit:
			if declares(pass, v, node.Type.Params, node.Type.Results) {
				return false
			}
		}
	}
	return true
}

// declares reports whether any of lists names v.
func declares(pass *analysis.Pass, v *types.Var, lists ...*ast.FieldList) bool {
	for _, list := range lists {
		if list == nil {
			continue
		}
		for _, f := range list.List {
			for _, name := range f.Names {
				if pass.TypesInfo.Defs[name] == v {
					return true
				}
			}
		}
	}
	return false
}

// read reports whether v is read at a site the deleted arguments do not hold.
// An assignment writes a variable without reading it, so it does not keep the
// declaration used.
func read(pass *analysis.Pass, written, gone map[*ast.Ident]struct{}, v *types.Var) bool {
	for use, obj := range pass.TypesInfo.Uses {
		if obj != v {
			continue
		}
		if _, ok := gone[use]; ok {
			continue
		}
		if _, ok := written[use]; !ok {
			return true
		}
	}
	return false
}

// writes returns the identifiers a plain assignment or a range assigns in
// file.
func writes(file *ast.File) map[*ast.Ident]struct{} {
	out := make(map[*ast.Ident]struct{})
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			// A compound assignment reads the target it writes.
			if astexpr.SelfReads(n) {
				return true
			}
			for _, lhs := range n.Lhs {
				if id, ok := ast.Unparen(lhs).(*ast.Ident); ok {
					out[id] = struct{}{}
				}
			}
		case *ast.RangeStmt:
			if n.Tok != token.DEFINE {
				for _, e := range []ast.Expr{n.Key, n.Value} {
					if id, ok := ast.Unparen(e).(*ast.Ident); ok {
						out[id] = struct{}{}
					}
				}
			}
		}
		return true
	})
	return out
}

// names returns the names the diagnostic uses for the call and for its target,
// using the file's name for a package and the declaring type's name for a
// method.
func names(pass *analysis.Pass, sel *ast.SelectorExpr, fn, target *types.Func) (_, _ string) {
	if id, ok := ast.Unparen(sel.X).(*ast.Ident); ok {
		if pkg, ok := pass.TypesInfo.Uses[id].(*types.PkgName); ok {
			return pkg.Name() + "." + fn.Name(),
				pkg.Name() + "." + target.Name()
		}
	}
	return funcLabel(fn), funcLabel(target)
}

// funcLabel names fn without a receiver's pointer, as in WaitGroup.Done.
func funcLabel(fn *types.Func) string {
	if recv := fn.Signature().Recv(); recv != nil {
		t := recv.Type()
		if pointer, ok := t.(*types.Pointer); ok {
			t = pointer.Elem()
		}
		if named, ok := t.(*types.Named); ok {
			return named.Obj().Name() + "." + fn.Name()
		}
	}
	if fn.Pkg() != nil {
		return fn.Pkg().Name() + "." + fn.Name()
	}
	return fn.Name()
}

// negative reports whether e is an integer literal below zero.
func negative(_ *analysis.Pass, e ast.Expr) bool {
	unary, ok := ast.Unparen(e).(*ast.UnaryExpr)
	if !ok || unary.Op != token.SUB {
		return false
	}
	lit, ok := ast.Unparen(unary.X).(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}
	v := constant.MakeFromLiteral(lit.Value, lit.Kind, 0)
	return v.Kind() == constant.Int && constant.Sign(v) > 0
}

// minusOne reports whether e is the integer literal -1.
func minusOne(_ *analysis.Pass, e ast.Expr) bool {
	unary, ok := ast.Unparen(e).(*ast.UnaryExpr)
	if !ok || unary.Op != token.SUB {
		return false
	}
	lit, ok := ast.Unparen(unary.X).(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}
	v := constant.MakeFromLiteral(lit.Value, lit.Kind, 0)
	return v.Kind() == constant.Int &&
		constant.Compare(v, token.EQL, constant.MakeInt64(1))
}

// zero reports whether e is an integer literal with the value zero.
func zero(_ *analysis.Pass, e ast.Expr) bool {
	lit, ok := ast.Unparen(e).(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}
	v := constant.MakeFromLiteral(lit.Value, lit.Kind, 0)
	return v.Kind() == constant.Int && constant.Sign(v) == 0
}

// nothing reports whether e is the nil literal.
func nothing(pass *analysis.Pass, e ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[ast.Unparen(e)]
	return ok && tv.IsNil()
}

// mapping reports whether e names the function of package unicode that the
// wrapper supplies.
func mapping(name string) func(*analysis.Pass, ast.Expr) bool {
	return func(pass *analysis.Pass, e ast.Expr) bool {
		_, fn, ok := uses.CalleeOf(pass, e)
		return ok && fn.Pkg() != nil && fn.Pkg().Path() == "unicode" &&
			fn.Signature().Recv() == nil && fn.Name() == name
	}
}
