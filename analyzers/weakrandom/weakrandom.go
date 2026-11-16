// Package weakrandom provides an analysis.Analyzer that reports a value from
// math/rand or math/rand/v2 that a token, a key, or a password receives.
//
// The math/rand package documentation says its outputs "might be easily
// predictable regardless of how it's seeded" and points to crypto/rand "[f]or
// random numbers suitable for security-sensitive work"
// (https://pkg.go.dev/math/rand). math/rand/v2 documents the same limits, and
// crypto/rand documents a "cryptographically secure random number generator".
//
// A value is reported when it reaches a token, a key, or a password by an
// assignment or declaration, a return from a function or a named result, an
// argument bound to a parameter, a conversion to a named type, a field of a
// composite literal, a send on a channel, or a Read that fills it. Assignments
// carry a value to another name. A conversion, an operator, or a formatting
// call carries it into a result, and a call to any other function does not. A
// composite literal records its fields and elements at their paths. Methods
// on a ChaCha8 are left alone; math/rand/v2 documents ChaCha8 as resistant to
// prediction.
//
//	func newToken() string {
//		return fmt.Sprint(rand.Intn(1000000))
//	}
//
// Every check and exemption is in the README.
//
// Report only: which generator to use depends on the author's requirements,
// and crypto/rand is the package the Go documentation names.
//
// The rule is adapted from gosec's G404.
package weakrandom

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "weakrandom",
	Doc:  "reports a weak random value used for a token, key, or password",
	Run:  run,
}

// source is a call to math/rand or math/rand/v2 whose value reaches a sink.
type source struct {
	pos, end token.Pos
	pkg      string
}

// path names a field, element, or dereference of a variable.
type path struct {
	obj   types.Object
	steps string
}

// checker holds the state of a file's analysis: the values known to carry a
// random result and the calls already reported.
type checker struct {
	pass      *analysis.Pass
	reported  map[token.Pos]bool
	taint     map[types.Object][]source
	paths     map[path][]source
	branch    bool
	pkgTaint  map[types.Object][]source
	pkgPaths  map[path][]source
	decls     map[*types.Func]*ast.FuncDecl
	summaries map[*types.Func]map[int][]source
	busy      map[*types.Func]bool
	silent    bool
	collect   map[int][]source
}

func run(pass *analysis.Pass) (any, error) {
	var (
		c = &checker{
			pass:      pass,
			reported:  make(map[token.Pos]bool),
			taint:     make(map[types.Object][]source),
			paths:     make(map[path][]source),
			decls:     make(map[*types.Func]*ast.FuncDecl),
			summaries: make(map[*types.Func]map[int][]source),
			busy:      make(map[*types.Func]bool),
		}
		specs []*ast.ValueSpec
		files []*ast.File
	)
	for _, file := range pass.Files {
		name := pass.Fset.File(file.Pos()).Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, file)
	}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil {
				continue
			}
			if obj, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func); ok {
				c.decls[obj] = fn
			}
		}
	}
	for _, file := range files {
		specs = append(specs, packageVars(file)...)
	}
	// Go initializes a package variable after the variables it refers to, so
	// a later declaration may supply an earlier one. Repeated passes reach
	// every dependency in a package that compiles.
	for range len(specs) {
		c.pkgTaint = copyTaint(c.taint)
		c.pkgPaths = copyPaths(c.paths)
		clear(c.summaries)
		for _, spec := range specs {
			c.valueSpec(spec)
		}
	}
	c.pkgTaint = copyTaint(c.taint)
	c.pkgPaths = copyPaths(c.paths)
	for _, file := range files {
		c.packageFuncs(file)
	}
	for _, file := range files {
		c.functions(file)
	}
	return nil, nil
}

// packageVars returns the package variables declared in file.
func packageVars(file *ast.File) (specs []*ast.ValueSpec) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			specs = append(specs, spec.(*ast.ValueSpec))
		}
	}
	return specs
}

// packageFuncs checks function literals in package variable initializers.
func (c *checker) packageFuncs(file *ast.File) {
	for _, spec := range packageVars(file) {
		for _, value := range spec.Values {
			ast.Inspect(value, func(n ast.Node) bool {
				if lit, ok := n.(*ast.FuncLit); ok {
					c.function(lit.Type, "", lit.Body)
					c.mergePackage()
					return false
				}
				return true
			})
		}
	}
}

// functions checks every function in file.
func (c *checker) functions(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if n.Body != nil {
				c.function(n.Type, n.Name.Name, n.Body)
				c.mergePackage()
			}
		}
		return true
	})
}

// mergePackage carries writes to package variables from the function just
// checked into the package state.
func (c *checker) mergePackage() {
	c.mergeTaint()
	c.mergePaths()
	clear(c.summaries)
}

// mergeTaint carries package variable taint from a function.
func (c *checker) mergeTaint() {
	for obj := range c.pkgTaint {
		if c.packageVar(obj) {
			if _, ok := c.taint[obj]; !ok {
				delete(c.pkgTaint, obj)
			}
		}
	}
	for obj, sources := range c.taint {
		if c.packageVar(obj) {
			c.pkgTaint[obj] = append([]source(nil), sources...)
		}
	}
}

// mergePaths carries package variable path taint from a function.
func (c *checker) mergePaths() {
	for p := range c.pkgPaths {
		if c.packageVar(p.obj) {
			if _, ok := c.paths[p]; !ok {
				delete(c.pkgPaths, p)
			}
		}
	}
	for p, sources := range c.paths {
		if c.packageVar(p.obj) {
			c.pkgPaths[p] = append([]source(nil), sources...)
		}
	}
}

// packageVar reports whether obj is a package variable.
func (c *checker) packageVar(obj types.Object) bool {
	return obj.Pkg() == c.pass.Pkg && obj.Parent() == c.pass.Pkg.Scope()
}

// copyTaint returns a copy of a taint map, with each source list copied.
func copyTaint(src map[types.Object][]source) map[types.Object][]source {
	out := make(map[types.Object][]source, len(src))
	for obj, sources := range src {
		out[obj] = append([]source(nil), sources...)
	}
	return out
}

// copyPaths returns a copy of a path taint map, with each source list copied.
func copyPaths(src map[path][]source) map[path][]source {
	out := make(map[path][]source, len(src))
	for p, sources := range src {
		out[p] = append([]source(nil), sources...)
	}
	return out
}

// function checks one function body. Nested function literals are checked
// where they appear, with the values their enclosing function carries.
func (c *checker) function(typ *ast.FuncType, name string, body *ast.BlockStmt) {
	c.taint = copyTaint(c.pkgTaint)
	c.paths = copyPaths(c.pkgPaths)
	c.body(typ, name, body)
}

// literal checks a function literal with a copy of its enclosing values.
func (c *checker) literal(lit *ast.FuncLit) {
	var (
		taint   = c.taint
		paths   = c.paths
		branch  = c.branch
		collect = c.collect
	)
	c.taint = copyTaint(taint)
	c.paths = copyPaths(paths)
	c.branch = false
	c.collect = nil
	c.body(lit.Type, "", lit.Body)
	c.taint, c.paths, c.branch, c.collect = taint, paths, branch, collect
}

// body walks one function body.
func (c *checker) body(typ *ast.FuncType, name string, body *ast.BlockStmt) {
	var (
		named, _ = sensitive(name)
		stack    []ast.Node
	)
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		c.branch = branched(stack, n)
		switch n := n.(type) {
		case *ast.FuncLit:
			c.literal(n)
			return false
		case *ast.AssignStmt:
			c.assign(n)
		case *ast.RangeStmt:
			c.rangeStmt(n)
		case *ast.ValueSpec:
			c.valueSpec(n)
		case *ast.ReturnStmt:
			c.returns(typ, named, n)
		case *ast.CallExpr:
			c.call(n)
		case *ast.CompositeLit:
			c.composite(n)
		case *ast.SendStmt:
			c.send(n)
		}
		stack = append(stack, n)
		return true
	})
}

// branched reports whether stack holds a condition or loop before a function
// boundary. An assignment there may not run, so it adds to a value rather than
// replacing it.
func branched(stack []ast.Node, node ast.Node) bool {
	for _, parent := range slices.Backward(stack) {
		switch parent := parent.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return false
		case *ast.IfStmt:
			if node.Pos() < parent.Body.Pos() {
				continue
			}
			return true
		case *ast.ForStmt:
			if node.Pos() < parent.Body.Pos() {
				continue
			}
			return true
		case *ast.RangeStmt:
			if node.Pos() < parent.Body.Pos() {
				continue
			}
			return true
		case *ast.SwitchStmt:
			if node.Pos() < parent.Body.Pos() {
				continue
			}
			return true
		case *ast.TypeSwitchStmt:
			if node.Pos() < parent.Body.Pos() {
				continue
			}
			return true
		case *ast.SelectStmt:
			return true
		}
	}
	return false
}

// returns checks the expressions of a return statement against the function
// name and the names of its results.
func (c *checker) returns(typ *ast.FuncType, named string, stmt *ast.ReturnStmt) {
	if c.collect != nil {
		for i, expr := range stmt.Results {
			c.collect[i] = append(c.collect[i], c.sources(expr)...)
		}
	}
	if len(stmt.Results) == 0 {
		c.namedResults(typ, named)
		return
	}
	for i, expr := range stmt.Results {
		kind := c.resultKind(typ, i)
		if kind == "" && named != "" && i == 0 {
			kind = named
		}
		if kind != "" {
			c.report(c.sources(expr), kind)
		}
	}
}

// namedResults reports named results that a naked return carries.
func (c *checker) namedResults(typ *ast.FuncType, named string) {
	if typ.Results == nil {
		return
	}
	var i int
	for _, field := range typ.Results.List {
		for _, name := range field.Names {
			kind := c.resultKind(typ, i)
			if kind == "" && named != "" && i == 0 {
				kind = named
			}
			obj := c.pass.TypesInfo.Defs[name]
			if c.collect != nil {
				c.collect[i] = append(c.collect[i], c.taint[obj]...)
			}
			if kind != "" {
				c.report(c.taint[obj], kind)
			}
			i++
		}
		if len(field.Names) == 0 {
			i++
		}
	}
}

// rangeSources returns the sources a range expression can yield.
func (c *checker) rangeSources(expr ast.Expr) []source {
	out := c.sources(expr)
	if p, ok := c.pathOf(expr); ok {
		for q, sources := range c.paths {
			if q.obj == p.obj && below(q.steps, p.steps) {
				out = append(out, sources...)
			}
		}
	}
	value := ast.Unparen(expr)
	if unary, ok := value.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		value = ast.Unparen(unary.X)
	}
	if lit, ok := value.(*ast.CompositeLit); ok {
		for _, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				out = append(out, c.sources(kv.Value)...)
				continue
			}
			out = append(out, c.sources(elt)...)
		}
	}
	return unique(out)
}

// rangeStmt checks the value a range clause assigns on each iteration.
func (c *checker) rangeStmt(stmt *ast.RangeStmt) {
	if stmt.Key != nil {
		c.store(stmt.Key, nil, false)
	}
	if stmt.Value == nil {
		return
	}
	sources := c.rangeSources(stmt.X)
	c.store(stmt.Value, sources, false)
	if kind, ok := c.targetKind(stmt.Value); ok {
		c.report(sources, kind)
	}
}

// assign checks an assignment and carries a random value into its targets.
func (c *checker) assign(stmt *ast.AssignStmt) {
	sources := make([][]source, len(stmt.Rhs))
	for i, expr := range stmt.Rhs {
		sources[i] = c.sources(expr)
	}
	add := c.branch || stmt.Tok != token.DEFINE && stmt.Tok != token.ASSIGN
	if len(stmt.Rhs) == len(stmt.Lhs) {
		for i, target := range stmt.Lhs {
			c.store(target, stored(sources[i], stmt.Rhs[i]), add)
			c.copyPaths(target, stmt.Rhs[i])
			c.storeComposite(target, stmt.Rhs[i])
			if kind, ok := c.targetKind(target); ok {
				c.report(stored(sources[i], stmt.Rhs[i]), kind)
			}
		}
		return
	}
	if len(stmt.Rhs) == 1 && len(stmt.Lhs) > 1 && c.multiAssign(stmt, add) {
		return
	}
	// A lone right-hand side can fill several targets, as a call that returns
	// two values does. Only the first target is paired with it.
	if len(stmt.Rhs) == 1 {
		c.store(stmt.Lhs[0], stored(sources[0], stmt.Rhs[0]), add)
		c.copyPaths(stmt.Lhs[0], stmt.Rhs[0])
		c.storeComposite(stmt.Lhs[0], stmt.Rhs[0])
		if kind, ok := c.targetKind(stmt.Lhs[0]); ok {
			c.report(stored(sources[0], stmt.Rhs[0]), kind)
		}
		for _, target := range stmt.Lhs[1:] {
			c.store(target, nil, false)
		}
		return
	}
	for _, target := range stmt.Lhs {
		c.store(target, nil, false)
	}
}

// multiAssign pairs per-result sources with several targets. It reports
// whether it handled the assignment.
func (c *checker) multiAssign(stmt *ast.AssignStmt, add bool) bool {
	call, ok := stmt.Rhs[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	results, ok := c.callResults(call)
	if !ok {
		return false
	}
	for i, target := range stmt.Lhs {
		c.store(target, results[i], add)
		if kind, ok := c.targetKind(target); ok {
			c.report(results[i], kind)
		}
	}
	return true
}

// multiSpec pairs per-result sources with several declared names. It reports
// whether it handled the declaration.
func (c *checker) multiSpec(spec *ast.ValueSpec) bool {
	call, ok := spec.Values[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	results, ok := c.callResults(call)
	if !ok {
		return false
	}
	for i, name := range spec.Names {
		c.storeIdent(name, results[i], false)
		if kind, ok := sensitive(name.Name); ok {
			c.report(results[i], kind)
		}
	}
	return true
}

// valueSpec checks a variable declaration and carries a random value into its
// names.
func (c *checker) valueSpec(spec *ast.ValueSpec) {
	if len(spec.Values) == 0 {
		return
	}
	sources := make([][]source, len(spec.Values))
	for i, expr := range spec.Values {
		sources[i] = c.sources(expr)
	}
	defer func() {
		for _, expr := range spec.Values {
			c.expression(expr)
		}
	}()
	if len(spec.Values) == len(spec.Names) {
		for i, name := range spec.Names {
			c.storeIdent(name, stored(sources[i], spec.Values[i]), false)
			c.copyPaths(name, spec.Values[i])
			c.storeComposite(name, spec.Values[i])
			if kind, ok := sensitive(name.Name); ok {
				c.report(stored(sources[i], spec.Values[i]), kind)
			}
		}
		return
	}
	if len(spec.Values) == 1 && len(spec.Names) > 1 && c.multiSpec(spec) {
		return
	}
	if len(spec.Values) == 1 {
		c.storeIdent(spec.Names[0], stored(sources[0], spec.Values[0]), false)
		c.copyPaths(spec.Names[0], spec.Values[0])
		c.storeComposite(spec.Names[0], spec.Values[0])
		if kind, ok := sensitive(spec.Names[0].Name); ok {
			c.report(stored(sources[0], spec.Values[0]), kind)
		}
		for _, name := range spec.Names[1:] {
			c.storeIdent(name, nil, false)
		}
		return
	}
	for _, name := range spec.Names {
		c.storeIdent(name, nil, false)
	}
}

// call checks the arguments of a call against the names of its parameters.
func (c *checker) call(call *ast.CallExpr) {
	if tv, ok := c.pass.TypesInfo.Types[call.Fun]; ok && tv.IsType() {
		if kind, ok := namedKind(tv.Type); ok {
			for _, arg := range call.Args {
				c.report(c.sources(arg), kind)
			}
		}
		return // a conversion has no parameters
	}
	c.read(call)
	typ := c.pass.TypesInfo.TypeOf(call.Fun)
	if typ == nil {
		return
	}
	sig, ok := typ.Underlying().(*types.Signature)
	if !ok {
		return
	}
	_, fn, _ := uses.CalleeOf(c.pass, call.Fun)
	switch {
	case fn != nil && fn.Pkg() == c.pass.Pkg:
		c.arguments(call, sig)
	case fn == nil && c.localCallable(call.Fun):
		c.arguments(call, sig)
	}
}

// arguments checks call arguments against a signature's parameter names.
func (c *checker) arguments(call *ast.CallExpr, sig *types.Signature) {
	for i, arg := range call.Args {
		if kind, ok := parameterKind(sig, i); ok {
			c.report(c.sources(arg), kind)
		}
	}
}

// localCallable reports whether fun is a function literal or a function value
// declared in this package.
func (c *checker) localCallable(fun ast.Expr) bool {
	switch fun := ast.Unparen(fun).(type) {
	case *ast.FuncLit:
		return true
	case *ast.Ident:
		obj, ok := c.pass.TypesInfo.ObjectOf(fun).(*types.Var)
		return ok && obj.Pkg() == c.pass.Pkg
	}
	return false
}

// read checks a weak Read, which fills its argument with random bytes.
func (c *checker) read(call *ast.CallExpr) {
	src, fn, ok := c.weakFunc(call)
	if !ok || fn.Name() != "Read" || len(call.Args) == 0 {
		return
	}
	var index int
	if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if c.methodExpression(sel) {
			index = 1
		}
	}
	if index >= len(call.Args) {
		return
	}
	arg := call.Args[index]
	if kind, ok := c.targetKind(arg); ok {
		c.report([]source{src}, kind)
	}
	c.fill(arg, src)
}

// fill carries a source into a buffer that a call writes through.
func (c *checker) fill(arg ast.Expr, src source) {
	arg = ast.Unparen(arg)
	if slice, ok := arg.(*ast.SliceExpr); ok {
		arg = ast.Unparen(slice.X)
	}
	if unary, ok := arg.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		arg = ast.Unparen(unary.X)
	}
	if id, ok := arg.(*ast.Ident); ok {
		c.storeIdent(id, []source{src}, true)
		return
	}
	if p, ok := c.pathOf(arg); ok {
		c.storePath(p, []source{src}, true)
	}
}

// methodExpression reports whether sel names a method expression.
func (c *checker) methodExpression(sel *ast.SelectorExpr) bool {
	selection := c.pass.TypesInfo.Selections[sel]
	return selection != nil && selection.Kind() == types.MethodExpr
}

// expression checks the sinks inside a top-level initializer.
func (c *checker) expression(expr ast.Expr) {
	ast.Inspect(expr, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			c.call(n)
		case *ast.CompositeLit:
			c.composite(n)
		}
		return true
	})
}

// composite checks the fields of a composite literal.
func (c *checker) composite(lit *ast.CompositeLit) {
	kind, fields := structure(c.pass.TypesInfo.TypeOf(lit))
	for i, elt := range lit.Elts {
		var (
			value = elt
			field string
		)
		switch elt := elt.(type) {
		case *ast.KeyValueExpr:
			value = elt.Value
			if name, ok := c.literalKind(elt.Key); ok {
				c.report(c.sources(value), name)
			}
			if fields == nil {
				break
			}
			if key, ok := elt.Key.(*ast.Ident); ok {
				field = key.Name
			}
		default:
			if fields != nil && i < fields.NumFields() {
				field = fields.Field(i).Name()
			}
		}
		if name, ok := sensitive(field); ok {
			c.report(c.sources(value), name)
		}
		if kind != "" {
			c.report(c.sources(value), kind)
		}
	}
}

// structure returns the kind a named type says and the fields it holds.
func structure(typ types.Type) (string, *types.Struct) {
	kind, _ := namedKind(typ)
	if typ == nil {
		return kind, nil
	}
	fields, _ := typ.Underlying().(*types.Struct)
	return kind, fields
}

// send checks the value sent on a channel.
func (c *checker) send(stmt *ast.SendStmt) {
	if kind, ok := c.targetKind(stmt.Chan); ok {
		c.report(c.sources(stmt.Value), kind)
	}
}

// sources returns the random calls whose values reach expr. An index selects a
// value from its base, so a random index does not carry its source into the
// result: keys[rand.Intn(len(keys))] yields a key, and the key is not random.
func (c *checker) sources(expr ast.Expr) []source {
	switch expr := expr.(type) {
	case *ast.Ident:
		obj := c.pass.TypesInfo.ObjectOf(expr)
		return c.taint[obj]
	case *ast.ParenExpr:
		return c.sources(expr.X)
	case *ast.CallExpr:
		return c.callSources(expr)
	case *ast.BinaryExpr:
		return append(c.sources(expr.X), c.sources(expr.Y)...)
	case *ast.UnaryExpr:
		return c.sources(expr.X)
	case *ast.IndexExpr:
		if value, ok := c.resolve(expr); ok {
			return c.sources(value)
		}
		if sources, ok := c.pathSources(expr); ok {
			return sources
		}
		if composite(expr.X) {
			return nil
		}
		return c.sources(expr.X)
	case *ast.IndexListExpr:
		return c.sources(expr.X)
	case *ast.SelectorExpr:
		if value, ok := c.resolve(expr); ok {
			return c.sources(value)
		}
		if sources, ok := c.pathSources(expr); ok {
			return sources
		}
		if composite(expr.X) {
			return nil
		}
		return c.sources(expr.X)
	case *ast.SliceExpr:
		if composite(expr.X) {
			return nil
		}
		return c.sources(expr.X)
	case *ast.StarExpr:
		if sources, ok := c.pathSources(expr); ok {
			return sources
		}
		return c.sources(expr.X)
	case *ast.TypeAssertExpr:
		return c.sources(expr.X)
	case *ast.CompositeLit:
		return nil
	case *ast.KeyValueExpr:
		return c.sources(expr.Value)
	}
	return nil
}

// summary returns the sources a function in this package returns, by result.
func (c *checker) summary(fn *types.Func) map[int][]source {
	if results, ok := c.summaries[fn]; ok {
		return results
	}
	if c.busy[fn] {
		return nil
	}
	decl := c.decls[fn]
	if decl == nil || decl.Body == nil {
		return nil
	}
	c.busy[fn] = true
	taint := c.taint
	paths := c.paths
	branch := c.branch
	silent := c.silent
	collect := c.collect
	c.taint = copyTaint(c.pkgTaint)
	c.paths = copyPaths(c.pkgPaths)
	c.branch = false
	c.silent = true
	c.collect = make(map[int][]source)
	c.body(decl.Type, "", decl.Body)
	out := c.collect
	c.taint, c.paths, c.branch = taint, paths, branch
	c.silent, c.collect = silent, collect
	delete(c.busy, fn)
	c.summaries[fn] = out
	return out
}

// callSources returns the sources a call passes to its result or carries
// through its arguments.
func (c *checker) callSources(call *ast.CallExpr) (out []source) {
	if lit, ok := ast.Unparen(call.Fun).(*ast.FuncLit); ok {
		return c.literalReturns(lit)
	}
	src, fn, ok := c.weakFunc(call)
	if ok {
		if fn.Name() == "Read" {
			return nil // Read fills its argument and returns a count
		}
		return []source{src}
	}
	_, local, ok := uses.CalleeOf(c.pass, call.Fun)
	if !c.carries(call) {
		if ok && local.Pkg() == c.pass.Pkg {
			return c.summary(local)[0]
		}
		return nil
	}
	for _, arg := range call.Args {
		out = append(out, c.sources(arg)...)
	}
	return out
}

// resolve follows a selector or index chain into composite literals and
// returns the selected value.
func (c *checker) resolve(expr ast.Expr) (ast.Expr, bool) {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.CompositeLit:
		return expr, true
	case *ast.UnaryExpr:
		if expr.Op == token.AND {
			return c.resolve(expr.X)
		}
	case *ast.SelectorExpr:
		base, ok := c.resolve(expr.X)
		if !ok {
			return nil, false
		}
		lit, ok := base.(*ast.CompositeLit)
		if !ok {
			return nil, false
		}
		return c.fieldValue(lit, expr.Sel.Name)
	case *ast.IndexExpr:
		base, ok := c.resolve(expr.X)
		if !ok {
			return nil, false
		}
		lit, ok := base.(*ast.CompositeLit)
		if !ok {
			return nil, false
		}
		return c.indexValue(lit, expr.Index)
	}
	return nil, false
}

// fieldValue returns the value a composite literal gives field name.
func (c *checker) fieldValue(lit *ast.CompositeLit, name string) (ast.Expr, bool) {
	_, fields := structure(c.pass.TypesInfo.TypeOf(lit))
	if fields == nil {
		return nil, false
	}
	for i, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == name {
				return kv.Value, true
			}
			continue
		}
		if i < fields.NumFields() && fields.Field(i).Name() == name {
			return elt, true
		}
	}
	return nil, false
}

// indexValue returns the value a composite literal gives an index or key.
func (c *checker) indexValue(lit *ast.CompositeLit, index ast.Expr) (ast.Expr, bool) {
	key, _ := c.indexKey(index)
	if key == "*" {
		return lit, true
	}
	for i, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if k, ok := c.indexKey(kv.Key); ok && k == key {
				return kv.Value, true
			}
			continue
		}
		if strconv.Itoa(i) == key {
			return elt, true
		}
	}
	return nil, false
}

// callResults returns per-result sources for a local function call.
func (c *checker) callResults(call *ast.CallExpr) (map[int][]source, bool) {
	_, fn, ok := uses.CalleeOf(c.pass, call.Fun)
	if !ok || fn.Pkg() != c.pass.Pkg {
		return nil, false
	}
	results := c.summary(fn)
	if results == nil {
		return nil, false
	}
	return results, true
}

// literalReturns returns the sources of the values a function literal
// returns.
func (c *checker) literalReturns(lit *ast.FuncLit) (out []source) {
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			for _, expr := range n.Results {
				out = append(out, c.sources(expr)...)
			}
		}
		return true
	})
	return out
}

// pathSources returns the taint recorded for a field, element, or
// dereference.
func (c *checker) pathSources(expr ast.Expr) ([]source, bool) {
	p, ok := c.pathOf(expr)
	if !ok {
		return nil, false
	}
	sources, ok := c.paths[p]
	return sources, ok
}

// pathOf returns the variable and the steps that name a field, constant
// element, or dereference.
func (c *checker) pathOf(expr ast.Expr) (path, bool) {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.Ident:
		obj := c.pass.TypesInfo.ObjectOf(expr)
		return path{obj: obj}, obj != nil
	case *ast.SelectorExpr:
		base := expr.X
		if star, ok := ast.Unparen(base).(*ast.StarExpr); ok {
			base = star.X
		}
		p, ok := c.pathOf(base)
		if !ok {
			return path{}, false
		}
		p.steps += "." + expr.Sel.Name
		return p, true
	case *ast.IndexExpr:
		p, ok := c.pathOf(expr.X)
		if !ok {
			return path{}, false
		}
		key, ok := c.indexKey(expr.Index)
		if !ok {
			return path{}, false
		}
		p.steps += "[" + key + "]"
		return p, true
	case *ast.StarExpr:
		p, ok := c.pathOf(expr.X)
		if !ok {
			return path{}, false
		}
		p.steps += ".*"
		return p, true
	}
	return path{}, false
}

// indexKey returns the constant value of an index.
func (c *checker) indexKey(expr ast.Expr) (string, bool) {
	tv, ok := c.pass.TypesInfo.Types[expr]
	if !ok || tv.Value == nil {
		return "*", true
	}
	return tv.Value.ExactString(), true
}

// weakFunc returns the source a call represents and the function it names.
func (c *checker) weakFunc(call *ast.CallExpr) (source, *types.Func, bool) {
	_, fn, ok := uses.CalleeOf(c.pass, call.Fun)
	if !ok || fn.Pkg() == nil {
		return source{}, nil, false
	}
	pkg := fn.Pkg().Path()
	if pkg != "math/rand" && pkg != "math/rand/v2" {
		return source{}, nil, false
	}
	if excluded(fn) {
		return source{}, nil, false
	}
	if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if c.chaCha8Source(sel.X) {
			return source{}, nil, false
		}
	}
	return source{pos: call.Pos(), end: call.End(), pkg: pkg}, fn, true
}

// excluded reports whether fn is a constructor, a configuration or
// serialization method, or a method on a ChaCha8 source.
func excluded(fn *types.Func) bool {
	switch fn.Name() {
	case "Seed", "MarshalBinary", "AppendBinary", "UnmarshalBinary":
		return true
	}
	if strings.HasPrefix(fn.Name(), "New") {
		return true
	}
	recv := fn.Signature().Recv()
	return recv != nil && chaCha8(recv.Type())
}

// chaCha8 reports whether t is the ChaCha8 source type, or a pointer to it.
func chaCha8(t types.Type) bool {
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "math/rand/v2" &&
		named.Obj().Name() == "ChaCha8"
}

// chaCha8Source reports whether expr is a ChaCha8 source, including a
// *rand.Rand built over one in the same expression.
func (c *checker) chaCha8Source(expr ast.Expr) bool {
	if chaCha8(c.pass.TypesInfo.TypeOf(expr)) {
		return true
	}
	switch expr := ast.Unparen(expr).(type) {
	case *ast.UnaryExpr:
		return expr.Op == token.AND && c.chaCha8Source(expr.X)
	case *ast.CallExpr:
		_, fn, ok := uses.CalleeOf(c.pass, expr.Fun)
		if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "math/rand/v2" {
			return false
		}
		return fn.Name() == "New" && len(expr.Args) > 0 &&
			c.chaCha8Source(expr.Args[0])
	}
	return false
}

// clearBelow drops paths below p, which an assignment to p replaces.
func (c *checker) clearBelow(p path) {
	for q := range c.paths {
		if q.obj == p.obj && below(q.steps, p.steps) {
			delete(c.paths, q)
		}
	}
}

// below reports whether steps names p or a field, element, or dereference of
// it.
func below(steps, base string) bool {
	if steps == base {
		return true
	}
	if !strings.HasPrefix(steps, base) {
		return false
	}
	rest := steps[len(base):]
	return strings.HasPrefix(rest, ".") || strings.HasPrefix(rest, "[")
}

// copyPaths carries field and element paths from one variable to another.
func (c *checker) copyPaths(target, expr ast.Expr) {
	to, ok := c.pathOf(target)
	if !ok {
		return
	}
	from, ok := c.pathOf(expr)
	if !ok {
		return
	}
	for p, sources := range c.paths {
		if p.obj != from.obj || !below(p.steps, from.steps) {
			continue
		}
		step := p.steps[len(from.steps):]
		c.storePath(path{obj: to.obj, steps: to.steps + step}, sources, false)
	}
}

// storeComposite records the fields and elements of a composite literal into
// the paths of the variable it initializes.
func (c *checker) storeComposite(target, expr ast.Expr) {
	base, ok := c.pathOf(target)
	if !ok {
		return
	}
	c.storeCompositeValue(base, expr)
}

// storeCompositeValue records the fields and elements of a composite value.
func (c *checker) storeCompositeValue(base path, expr ast.Expr) {
	expr = ast.Unparen(expr)
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = ast.Unparen(unary.X)
	}
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return
	}
	_, fields := structure(c.pass.TypesInfo.TypeOf(lit))
	for i, elt := range lit.Elts {
		var (
			step  string
			value = elt
		)
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			value = kv.Value
			if fields != nil {
				if key, ok := kv.Key.(*ast.Ident); ok {
					step = "." + key.Name
				}
			} else if key, ok := c.indexKey(kv.Key); ok {
				step = "[" + key + "]"
			}
		} else if fields != nil && i < fields.NumFields() {
			step = "." + fields.Field(i).Name()
		} else if fields == nil {
			step = "[" + strconv.Itoa(i) + "]"
		}
		if step == "" {
			continue
		}
		p := path{obj: base.obj, steps: base.steps + step}
		c.clearBelow(p)
		if composite(value) {
			c.storeCompositeValue(p, value)
			continue
		}
		c.storePath(p, c.sources(value), false)
	}
}

// stored drops the taint of a composite literal: the container does not carry
// a source from one of its fields or elements.
func stored(sources []source, expr ast.Expr) []source {
	if composite(expr) {
		return nil
	}
	return sources
}

// composite reports whether expr is a composite literal, or its address.
func composite(expr ast.Expr) bool {
	expr = ast.Unparen(expr)
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = ast.Unparen(unary.X)
	}
	_, ok := expr.(*ast.CompositeLit)
	return ok
}

// carries reports whether a call's result is built from its arguments. A
// conversion and a formatting function carry a value; any other call ends it,
// so len(fmt.Sprint(rand.Intn(10))) has no random value in its result.
func (c *checker) carries(call *ast.CallExpr) bool {
	if tv, ok := c.pass.TypesInfo.Types[call.Fun]; ok && tv.IsType() {
		return true
	}
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		switch c.pass.TypesInfo.Uses[id] {
		case types.Universe.Lookup("append"), types.Universe.Lookup("complex"):
			return true
		}
	}
	_, fn, ok := uses.CalleeOf(c.pass, call.Fun)
	if !ok || fn.Pkg() == nil {
		return false
	}
	switch fn.Pkg().Path() {
	case "fmt":
		switch fn.Name() {
		case "Sprint", "Sprintf", "Sprintln", "Append", "Appendf",
			"Appendln", "Errorf":
			return true
		}
	case "strconv":
		switch fn.Name() {
		case "Itoa", "FormatInt", "FormatUint", "FormatFloat",
			"FormatComplex", "FormatBool", "AppendInt", "AppendUint",
			"AppendFloat", "AppendBool", "Quote", "QuoteToASCII",
			"QuoteToGraphic", "QuoteRune", "QuoteRuneToASCII",
			"QuoteRuneToGraphic", "AppendQuote", "AppendQuoteToASCII",
			"AppendQuoteToGraphic", "AppendQuoteRune",
			"AppendQuoteRuneToASCII", "AppendQuoteRuneToGraphic":
			return true
		}
	case "encoding/base32", "encoding/base64", "encoding/hex":
		return fn.Name() == "EncodeToString"
	}
	return false
}

// store carries sources into the variable target names.
func (c *checker) store(target ast.Expr, sources []source, add bool) {
	if id, ok := ast.Unparen(target).(*ast.Ident); ok {
		c.storeIdent(id, sources, add)
		return
	}
	if p, ok := c.pathOf(target); ok {
		c.storePath(p, sources, add)
	}
}

// storePath carries sources into a field, element, or dereference.
func (c *checker) storePath(p path, sources []source, add bool) {
	if c.paths == nil {
		return
	}
	if !add {
		c.clearBelow(p)
	}
	if add {
		sources = append(append([]source{}, c.paths[p]...), sources...)
	}
	if len(sources) == 0 {
		c.paths[p] = nil
		return
	}
	c.paths[p] = unique(sources)
}

// storeIdent carries sources into name.
func (c *checker) storeIdent(name *ast.Ident, sources []source, add bool) {
	if name.Name == "_" || c.taint == nil {
		return
	}
	obj := c.pass.TypesInfo.ObjectOf(name)
	if obj == nil {
		return
	}
	if add {
		sources = append(append([]source{}, c.taint[obj]...), sources...)
	} else {
		for p := range c.paths {
			if p.obj == obj {
				delete(c.paths, p)
			}
		}
	}
	if len(sources) == 0 {
		delete(c.taint, obj)
		return
	}
	c.taint[obj] = unique(sources)
}

// report reports each source once.
func (c *checker) report(sources []source, kind string) {
	if c.silent {
		return
	}
	for _, src := range unique(sources) {
		if c.reported[src.pos] {
			continue
		}
		c.reported[src.pos] = true
		c.pass.Report(analysis.Diagnostic{
			Pos: src.pos,
			End: src.end,
			Message: src.pkg +
				" is not suitable for security-sensitive work; use crypto/rand for a " +
				kind,
		})
	}
}

// unique drops repeated sources, which a value read twice can produce.
func unique(sources []source) []source {
	var (
		seen = make(map[token.Pos]struct{}, len(sources))
		out  = make([]source, 0, len(sources))
	)
	for _, src := range sources {
		if _, ok := seen[src.pos]; ok {
			continue
		}
		seen[src.pos] = struct{}{}
		out = append(out, src)
	}
	return out
}

// literalKind returns the kind a string constant names.
func (c *checker) literalKind(expr ast.Expr) (string, bool) {
	tv, ok := c.pass.TypesInfo.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return sensitive(constant.StringVal(tv.Value))
}

// kindOf returns the kind a value's named type says.
func (c *checker) kindOf(expr ast.Expr) (string, bool) {
	return namedKind(c.pass.TypesInfo.TypeOf(expr))
}

// targetKind returns the kind a target is named for.
func (c *checker) targetKind(expr ast.Expr) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Ident:
		if kind, ok := sensitive(expr.Name); ok {
			return kind, true
		}
		return c.kindOf(expr)
	case *ast.SelectorExpr:
		if kind, ok := sensitive(expr.Sel.Name); ok {
			return kind, true
		}
		if kind, ok := c.kindOf(expr); ok {
			return kind, true
		}
		return c.targetKind(expr.X)
	case *ast.IndexExpr:
		if kind, ok := c.literalKind(expr.Index); ok {
			return kind, true
		}
		if kind, ok := c.kindOf(expr); ok {
			return kind, true
		}
		return c.targetKind(expr.X)
	case *ast.SliceExpr:
		return c.targetKind(expr.X)
	case *ast.IndexListExpr:
		return c.targetKind(expr.X)
	case *ast.StarExpr:
		if kind, ok := c.kindOf(expr); ok {
			return kind, true
		}
		return c.targetKind(expr.X)
	case *ast.ParenExpr:
		return c.targetKind(expr.X)
	}
	return "", false
}

// parameterKind returns the kind of the parameter at index i. The last
// parameter of a variadic signature takes every argument from i on.
func parameterKind(sig *types.Signature, i int) (string, bool) {
	params := sig.Params()
	if params.Len() == 0 {
		return "", false
	}
	if sig.Variadic() && i >= params.Len()-1 {
		return varKind(params.At(params.Len() - 1))
	}
	if i < params.Len() {
		return varKind(params.At(i))
	}
	return "", false
}

// varKind returns the kind a variable's name or type says.
func varKind(v *types.Var) (string, bool) {
	if kind, ok := sensitive(v.Name()); ok {
		return kind, true
	}
	return namedKind(v.Type())
}

// resultKind returns the kind of the result at index i, when the function
// names it.
func (c *checker) resultKind(typ *ast.FuncType, index int) string {
	if typ.Results == nil {
		return ""
	}
	var i int
	for _, field := range typ.Results.List {
		if len(field.Names) == 0 {
			if i == index {
				kind, _ := namedKind(c.pass.TypesInfo.TypeOf(field.Type))
				return kind
			}
			i++
			continue
		}
		for _, name := range field.Names {
			if i == index {
				if kind, ok := sensitive(name.Name); ok {
					return kind
				}
				kind, _ := namedKind(c.pass.TypesInfo.TypeOf(field.Type))
				return kind
			}
			i++
		}
	}
	return ""
}

// namedKind returns the kind a named type is called.
func namedKind(typ types.Type) (string, bool) {
	if typ == nil {
		return "", false
	}
	t := types.Unalias(typ)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil {
		return "", false
	}
	return sensitiveType(named.Obj().Name())
}

// sensitiveType returns the kind a type name is called, when the whole name
// is one of the three words.
func sensitiveType(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "token", "tokens":
		return "token", true
	case "key", "keys":
		return "key", true
	case "password", "passwords":
		return "password", true
	}
	return "", false
}

// sensitive returns the kind a name says: a token, a key, or a password. The
// name is split at case changes and non-letters, so apiKey names a key and
// monkey does not.
func sensitive(name string) (string, bool) {
	for _, word := range words(name) {
		switch strings.ToLower(word) {
		case "token", "tokens":
			return "token", true
		case "key", "keys":
			return "key", true
		case "password", "passwords":
			return "password", true
		}
	}
	return "", false
}

// words splits an identifier at case changes and non-letters.
func words(name string) (out []string) {
	var start int
	for i := 1; i < len(name); i++ {
		prev, cur := name[i-1], name[i]
		if !letter(cur) || !letter(prev) || beginsWord(name, i) {
			out = append(out, name[start:i])
			start = i
		}
	}
	if start < len(name) {
		out = append(out, name[start:])
	}
	return out
}

// beginsWord reports whether the letter at i starts a new word: an upper-case
// letter after a lower-case one, or the last capital of a run before a
// lower-case one.
func beginsWord(name string, i int) bool {
	prev, cur := name[i-1], name[i]
	if !upper(cur) {
		return false
	}
	if lower(prev) {
		return true
	}
	return upper(prev) && i+1 < len(name) && lower(name[i+1])
}

func letter(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

func upper(c byte) bool { return 'A' <= c && c <= 'Z' }

func lower(c byte) bool { return 'a' <= c && c <= 'z' }
