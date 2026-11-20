// Package osremoveall provides an analysis.Analyzer that reports os.Remove or
// os.RemoveAll on a path that a variable leaves at the filesystem root.
//
// os.RemoveAll removes path and any children it contains
// (https://pkg.go.dev/os#RemoveAll), and os.Remove removes the file or empty
// directory named by path (https://pkg.go.dev/os#Remove). A path built as
// dir + "/" names dir/ when dir has a value and / when dir is empty, so the
// call deletes outside the tree the caller named.
//
// The rule reads a string concatenation or a path/filepath.Join or path.Join
// call. It reports when the path is the filesystem root once the variables are
// empty. A literal directory name keeps the path below the
// root, as /cache does; a leading separator and a literal such as /.. can
// leave the root. A plain variable and a path returned by any other call are
// left alone.
//
// A guard on the whole path proves it is not the root. A guard on the only
// variable also settles a path with no leading separator and no dot
// component. A string constant assigned to the last variable settles the path
// when it keeps the path below the root. The rule reads if and else branches,
// and preceding if statements that end their branch. A later write to the
// variable ends the guard.
//
// No fix is offered, because the repair is either a check that the variable is
// neither empty nor the root, or a path that does not start at the root. The
// rule is adapted from staticcheck's SA9007.
package osremoveall

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "osremoveall",
	Doc:  "reports a removal at the filesystem root caused by an empty variable",
	Run:  run,
}

// checker holds the state of one pass.
type checker struct {
	pass     *analysis.Pass
	aliases  map[types.Object]ast.Expr
	closures map[types.Object]bool
	values   map[types.Object]string
}

// guard lists the expressions an if condition proves non-empty and non-root.
type guard struct {
	nonempty []ast.Expr
	nonroot  []ast.Expr
	plain    []ast.Expr
}

// part is one element of a path: a constant string, or an expression.
type part struct {
	expr ast.Expr
	text string
	ok   bool // const
}

// path is a reported path and the variables it holds.
type path struct {
	expr     ast.Expr
	vars     []ast.Expr
	leading  bool
	complex  bool
	tail     bool
	tailText string
	concat   bool
}

func run(pass *analysis.Pass) (any, error) {
	c := &checker{
		pass:     pass,
		aliases:  make(map[types.Object]ast.Expr),
		closures: make(map[types.Object]bool),
		values:   make(map[types.Object]string),
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Body != nil {
					c.walkBlock(decl.Body, guard{})
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, expr := range value.Values {
						c.walkExpr(expr, guard{})
					}
				}
			}
		}
	}
	return nil, nil
}

// walkBlock walks a block in source order and returns the guards that hold
// after it. A guard stops holding when a later statement writes the variable.
func (c *checker) walkBlock(block *ast.BlockStmt, facts guard) guard {
	for _, stmt := range block.List {
		facts = c.walkStmt(stmt, facts)
	}
	return facts
}

// walkStmt walks one statement and returns the guards that hold after it.
func (c *checker) walkStmt(stmt ast.Stmt, facts guard) guard {
	switch stmt := stmt.(type) {
	case *ast.BlockStmt:
		return c.walkBlock(stmt, facts)
	case *ast.IfStmt:
		return c.walkIf(stmt, facts)
	case *ast.ForStmt:
		return c.walkFor(stmt, facts)
	case *ast.RangeStmt:
		return c.walkRange(stmt, facts)
	case *ast.SwitchStmt:
		return c.walkSwitch(stmt, facts)
	case *ast.TypeSwitchStmt:
		return c.walkTypeSwitch(stmt, facts)
	case *ast.SelectStmt:
		return c.walkSelect(stmt, facts)
	case *ast.LabeledStmt:
		return c.walkStmt(stmt.Stmt, facts)
	case *ast.AssignStmt:
		for _, expr := range stmt.Rhs {
			c.walkExpr(expr, facts)
		}
		if stmt.Tok == token.ASSIGN || stmt.Tok == token.DEFINE {
			facts = c.assigned(stmt.Lhs, stmt.Rhs, facts)
		} else if slices.ContainsFunc(stmt.Lhs, isStar) {
			facts = guard{}
		} else {
			c.forget(stmt.Lhs)
			facts = facts.without(c.pass, stmt.Lhs)
		}
		return c.withoutEffects(facts, stmt.Rhs)
	case *ast.IncDecStmt:
		c.walkExpr(stmt.X, facts)
		return facts.without(c.pass, []ast.Expr{stmt.X})
	case *ast.DeclStmt:
		return c.walkDecl(stmt, facts)
	case *ast.ReturnStmt:
		for _, expr := range stmt.Results {
			c.walkExpr(expr, facts)
		}
		return facts
	case *ast.ExprStmt:
		c.walkExpr(stmt.X, facts)
		return c.withoutEffects(facts, []ast.Expr{stmt.X})
	case *ast.SendStmt:
		c.walkExpr(stmt.Chan, facts)
		c.walkExpr(stmt.Value, facts)
		return c.withoutEffects(facts, []ast.Expr{stmt.Chan, stmt.Value})
	case *ast.GoStmt:
		c.walkExpr(stmt.Call, facts)
		return c.withoutEffects(facts, []ast.Expr{stmt.Call})
	case *ast.DeferStmt:
		c.walkExpr(stmt.Call, facts)
		return c.withoutEffects(facts, stmt.Call.Args)
	case *ast.BranchStmt:
		if stmt.Tok == token.GOTO {
			return guard{}
		}
		return facts
	}
	return facts
}

// walkDecl walks the values of a declaration and returns the guards that hold
// after it.
func (c *checker) walkDecl(stmt *ast.DeclStmt, facts guard) guard {
	decl, ok := stmt.Decl.(*ast.GenDecl)
	if !ok {
		return facts
	}
	for _, spec := range decl.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		names := make([]ast.Expr, len(value.Names))
		for i, name := range value.Names {
			names[i] = name
		}
		for _, expr := range value.Values {
			c.walkExpr(expr, facts)
		}
		facts = c.withoutEffects(facts, value.Values)
		if len(names) == len(value.Values) {
			facts = c.assigned(names, value.Values, facts)
		}
	}
	return facts
}

// assigned returns the guards that hold after lhs receives rhs.
func (c *checker) assigned(lhs, rhs []ast.Expr, facts guard) guard {
	if slices.ContainsFunc(lhs, isStar) {
		return guard{}
	}
	facts = facts.without(c.pass, lhs)
	if len(lhs) != len(rhs) {
		return facts
	}
	for i, expr := range rhs {
		c.alias(lhs[i], expr)
		text, ok := stringConst(c.pass, expr)
		c.value(lhs[i], text, ok)
		if !ok || isRoot(text) || isBlank(lhs[i]) {
			continue
		}
		facts = facts.allow(lhs[i])
		if belowRoot(text) {
			facts = facts.fixed(lhs[i])
		}
	}
	return facts
}

// value records the constant string lhs receives.
func (c *checker) value(lhs ast.Expr, text string, ok bool) {
	obj := object(c.pass, lhs)
	if obj == nil {
		return
	}
	if ok {
		c.values[obj] = text
		return
	}
	delete(c.values, obj)
}

// forget drops the values of the expressions written.
func (c *checker) forget(exprs []ast.Expr) {
	for _, expr := range exprs {
		if obj := object(c.pass, expr); obj != nil {
			delete(c.values, obj)
		}
	}
}

// object returns the object e names, when e is an identifier.
func object(pass *analysis.Pass, e ast.Expr) types.Object {
	ident, ok := ast.Unparen(e).(*ast.Ident)
	if !ok || isBlank(ident) {
		return nil
	}
	return pass.TypesInfo.ObjectOf(ident)
}

// alias records when lhs receives the address of a value or a function
// literal.
func (c *checker) alias(lhs, rhs ast.Expr) {
	ident, ok := ast.Unparen(lhs).(*ast.Ident)
	if !ok || isBlank(ident) {
		return
	}
	obj := c.pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return
	}
	delete(c.aliases, obj)
	delete(c.closures, obj)
	switch rhs := ast.Unparen(rhs).(type) {
	case *ast.UnaryExpr:
		if rhs.Op == token.AND {
			c.aliases[obj] = rhs.X
		}
	case *ast.FuncLit:
		c.closures[obj] = true
	case *ast.Ident:
		src := c.pass.TypesInfo.ObjectOf(rhs)
		if src == nil {
			return
		}
		if expr, ok := c.aliases[src]; ok {
			c.aliases[obj] = expr
		}
		if c.closures[src] {
			c.closures[obj] = true
		}
	}
}

// withoutEffects drops the facts a call in exprs can invalidate.
func (c *checker) withoutEffects(facts guard, exprs []ast.Expr) guard {
	for _, expr := range exprs {
		if c.clearsAll(expr) {
			facts = guard{}
		} else if c.effectful(expr) {
			facts = facts.keepLocal(c.pass)
		}
		var writes []ast.Expr
		writes = append(writes, written(expr)...)
		writes = append(writes, c.addressed(expr)...)
		facts = facts.without(c.pass, writes)
	}
	return facts
}

// effectful reports whether node calls a function that is not a path helper.
func (c *checker) effectful(node ast.Node) bool {
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !astexpr.Effectful(c.pass, call) || pureCall(c.pass, call) {
			return true
		}
		found = true
		return false
	})
	return found
}

// pureCall reports whether call only builds a path.
func pureCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if join(pass, call) {
		return true
	}
	_, ok := cleanArg(pass, call)
	return ok
}

// clearsAll reports whether node calls a function value or names a closure,
// which may write any variable the value captured.
func (c *checker) clearsAll(node ast.Node) bool {
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if c.closures[c.pass.TypesInfo.ObjectOf(n)] {
				found = true
			}
		case *ast.CallExpr:
			if !astexpr.Effectful(c.pass, n) {
				break
			}
			if typeutil.StaticCallee(c.pass.TypesInfo, n) == nil {
				found = true
			}
		}
		return !found
	})
	return found
}

// isStar reports whether e is a dereference.
func isStar(e ast.Expr) bool {
	_, ok := ast.Unparen(e).(*ast.StarExpr)
	return ok
}

// isBlank reports whether e is the blank identifier.
func isBlank(e ast.Expr) bool {
	ident, ok := ast.Unparen(e).(*ast.Ident)
	return ok && ident.Name == "_"
}

// allSeparators reports whether text holds one or more path separators and
// nothing else.
func allSeparators(text string) bool {
	if text == "" {
		return false
	}
	for i := 0; i < len(text); i++ {
		if !os.IsPathSeparator(text[i]) {
			return false
		}
	}
	return true
}

// rootPath reports whether text is a non-empty path at the filesystem root.
func rootPath(text string) bool { return text != "" && isRoot(text) }

// belowRoot reports whether text keeps the path below the root when it follows
// a separator, as "cache" does and ".." does not.
func belowRoot(text string) bool {
	return !isRoot(filepath.Clean(string(filepath.Separator) + text))
}

// isRoot reports whether text names the filesystem root or is empty.
func isRoot(text string) bool {
	if text == "" {
		return true
	}
	var (
		clean  = filepath.Clean(text)
		volume = filepath.VolumeName(clean)
		rest   = clean[len(volume):]
		seps   = string(filepath.Separator) + "/"
	)
	return strings.Trim(rest, seps) == ""
}

// condition returns the facts cond proves after its own effects.
func (c *checker) condition(cond ast.Expr, truth bool) guard {
	return c.withoutEffects(factsOf(c.pass, cond, truth), []ast.Expr{cond})
}

// walkIf walks an if statement and returns the guards that hold after it.
func (c *checker) walkIf(stmt *ast.IfStmt, facts guard) guard {
	if stmt.Init != nil {
		facts = c.walkStmt(stmt.Init, facts)
	}
	c.walkExpr(stmt.Cond, facts)
	facts = c.withoutEffects(facts, []ast.Expr{stmt.Cond})
	var (
		thenFacts = facts.merge(c.condition(stmt.Cond, true))
		elseFacts = facts.merge(c.condition(stmt.Cond, false))
		bodyFacts = c.walkBlock(stmt.Body, thenFacts)
	)
	if stmt.Else == nil {
		if c.terminates(stmt.Body) {
			return elseFacts
		}
		return bodyFacts.intersect(c.pass, elseFacts)
	}
	elseFacts = c.walkStmt(stmt.Else, elseFacts)
	switch {
	case c.terminates(stmt.Body) && c.terminates(stmt.Else):
		return facts
	case c.terminates(stmt.Body):
		return elseFacts
	case c.terminates(stmt.Else):
		return bodyFacts
	default:
		return bodyFacts.intersect(c.pass, elseFacts)
	}
}

// walkFor walks a for statement and returns the guards that hold after it.
func (c *checker) walkFor(stmt *ast.ForStmt, facts guard) guard {
	if stmt.Init != nil {
		facts = c.walkStmt(stmt.Init, facts)
	}
	if stmt.Cond != nil {
		c.walkExpr(stmt.Cond, facts)
		facts = c.withoutEffects(facts, []ast.Expr{stmt.Cond})
	}
	bodyFacts := facts
	if stmt.Cond != nil {
		bodyFacts = bodyFacts.merge(c.condition(stmt.Cond, true))
	}
	bodyAfter := c.walkBlock(stmt.Body, bodyFacts)
	if stmt.Post != nil {
		c.walkStmt(stmt.Post, bodyFacts)
	}
	facts = facts.intersect(c.pass, bodyAfter)
	return facts.without(c.pass, written(stmt))
}

// walkRange walks a range statement and returns the guards that hold after it.
func (c *checker) walkRange(stmt *ast.RangeStmt, facts guard) guard {
	c.walkExpr(stmt.X, facts)
	facts = c.withoutEffects(facts, []ast.Expr{stmt.X})
	if stmt.Key != nil {
		facts = facts.without(c.pass, []ast.Expr{stmt.Key})
	}
	if stmt.Value != nil {
		facts = facts.without(c.pass, []ast.Expr{stmt.Value})
	}
	bodyAfter := c.walkBlock(stmt.Body, facts)
	facts = facts.intersect(c.pass, bodyAfter)
	return facts.without(c.pass, written(stmt))
}

// walkSwitch walks an expression switch and returns the guards that hold after
// it.
func (c *checker) walkSwitch(stmt *ast.SwitchStmt, facts guard) guard {
	if stmt.Init != nil {
		facts = c.walkStmt(stmt.Init, facts)
	}
	if stmt.Tag != nil {
		c.walkExpr(stmt.Tag, facts)
		facts = c.withoutEffects(facts, []ast.Expr{stmt.Tag})
	}
	var caseFalse guard
	if stmt.Tag == nil {
		caseFalse = c.falseCases(stmt.Body)
		caseFalse = c.withoutEffects(caseFalse, caseExprs(stmt.Body))
	}
	return c.cases(stmt, facts, caseFalse)
}

// cases walks the case clauses and returns the facts that hold after them.
func (c *checker) cases(stmt *ast.SwitchStmt, facts, caseFalse guard) guard {
	var (
		fall       []guard
		hasDefault bool
		walkCase   = func(clause *ast.CaseClause, facts guard) guard {
			for _, expr := range clause.List {
				c.walkExpr(expr, facts)
			}
			facts = c.withoutEffects(facts, clause.List)
			if stmt.Tag != nil {
				return facts
			}
			switch len(clause.List) {
			case 0:
				return facts.merge(caseFalse)
			case 1:
				return facts.merge(factsOf(c.pass, clause.List[0], true))
			}
			return facts
		}
	)
	for _, node := range stmt.Body.List {
		clause, ok := node.(*ast.CaseClause)
		if !ok {
			continue
		}
		caseFacts := walkCase(clause, facts)
		body := &ast.BlockStmt{List: clause.Body}
		after := c.walkBlock(body, caseFacts)
		if !c.caseStops(body) {
			fall = append(fall, after)
		}
		if len(clause.List) == 0 {
			hasDefault = true
		}
		facts = c.withoutEffects(facts, clause.List)
	}
	if !hasDefault {
		fall = append(fall, facts.merge(caseFalse))
	}
	if len(fall) == 0 {
		return facts
	}
	common := fall[0]
	for _, after := range fall[1:] {
		common = common.intersect(c.pass, after)
	}
	return common
}

// caseExprs returns every case expression in body.
func caseExprs(body *ast.BlockStmt) (out []ast.Expr) {
	for _, stmt := range body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		out = append(out, clause.List...)
	}
	return out
}

// falseCases returns the facts that hold when no case expression matches.
func (c *checker) falseCases(body *ast.BlockStmt) (out guard) {
	for _, stmt := range body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, expr := range clause.List {
			out = out.merge(factsOf(c.pass, expr, false))
		}
	}
	return out
}

// caseStops reports whether stmt leaves a case without reaching the statements
// after the switch.
func (c *checker) caseStops(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return stmt.Tok == token.CONTINUE
	case *ast.BlockStmt:
		return len(stmt.List) > 0 && c.caseStops(stmt.List[len(stmt.List)-1])
	case *ast.IfStmt:
		if stmt.Else == nil {
			return false
		}
		return c.caseStops(stmt.Body) && c.caseStops(stmt.Else)
	case *ast.LabeledStmt:
		return c.caseStops(stmt.Stmt)
	case *ast.ExprStmt:
		return isPanic(c.pass, stmt.X)
	}
	return false
}

// walkTypeSwitch walks a type switch and returns the guards that hold after
// it.
func (c *checker) walkTypeSwitch(stmt *ast.TypeSwitchStmt, facts guard) guard {
	if stmt.Init != nil {
		facts = c.walkStmt(stmt.Init, facts)
	}
	facts = c.walkStmt(stmt.Assign, facts)
	for _, clause := range stmt.Body.List {
		clause, ok := clause.(*ast.CaseClause)
		if !ok {
			continue
		}
		body := &ast.BlockStmt{List: clause.Body}
		after := c.walkBlock(body, facts)
		if !c.caseStops(body) {
			facts = facts.intersect(c.pass, after)
		}
	}
	return facts.without(c.pass, written(stmt))
}

// walkSelect walks a select statement and returns the guards that hold after
// it.
func (c *checker) walkSelect(stmt *ast.SelectStmt, facts guard) guard {
	for _, clause := range stmt.Body.List {
		clause, ok := clause.(*ast.CommClause)
		if !ok {
			continue
		}
		commFacts := facts
		if clause.Comm != nil {
			commFacts = c.walkStmt(clause.Comm, facts)
		}
		body := &ast.BlockStmt{List: clause.Body}
		after := c.walkBlock(body, commFacts)
		if !c.caseStops(body) {
			facts = facts.intersect(c.pass, after)
		}
	}
	return facts.without(c.pass, written(stmt))
}

// walkExpr walks every call in e. A function literal starts a new body, which
// sees none of the guards around it: the literal can run after the variable
// changes.
func (c *checker) walkExpr(e ast.Expr, facts guard) {
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			c.walkBlock(n.Body, guard{})
			return false
		case *ast.CallExpr:
			c.checkCall(n, facts)
		}
		return true
	})
}

// checkCall reports a remove call whose path an empty variable leaves at the
// filesystem root.
func (c *checker) checkCall(call *ast.CallExpr, facts guard) {
	fn := typeutil.StaticCallee(c.pass.TypesInfo, call)
	if fn == nil {
		return
	}
	switch fn.FullName() {
	case "os.Remove", "os.RemoveAll":
	default:
		return
	}
	if len(call.Args) != 1 {
		return
	}
	path, ok := c.path(call.Args[0])
	if !ok || c.safe(path, facts) {
		return
	}
	name := "os." + fn.Name()
	c.pass.Report(analysis.Diagnostic{
		Pos: path.expr.Pos(),
		End: path.expr.End(),
		Message: name + " path built from a variable may be " +
			"the filesystem root",
	})
}

// path returns the path argument when an empty variable makes it the root.
func (c *checker) path(arg ast.Expr) (path, bool) {
	var (
		tail      bool
		tailText  strings.Builder
		parts, ok = c.parts(arg)
	)
	if !ok || len(parts) == 0 {
		return path{}, false
	}
	first := slices.IndexFunc(parts, func(p part) bool {
		return !p.ok || p.text != ""
	})
	if first < 0 {
		return path{}, false
	}
	empty, ok := c.text(arg)
	if !ok {
		return path{}, false
	}
	summary := summarize(parts)
	if len(summary.vars) == 0 || !rootPath(empty) {
		return path{}, false
	}
	for _, p := range parts[summary.last+1:] {
		if !p.ok {
			continue
		}
		tailText.WriteString(p.text)
		if p.text != "" && !allSeparators(p.text) {
			tail = true
		}
	}
	_, concat := ast.Unparen(arg).(*ast.BinaryExpr)
	return path{
		expr:     ast.Unparen(arg),
		vars:     summary.vars,
		leading:  parts[first].ok,
		complex:  summary.complex,
		tail:     tail,
		tailText: tailText.String(),
		concat:   concat,
	}, true
}

// summary holds the facts a path's parts give.
type summary struct {
	vars     []ast.Expr
	last     int
	complex  bool
	hasConst bool
}

// summarize reads the variables and constants of parts.
func summarize(parts []part) (out summary) {
	out.last = -1
	for i, p := range parts {
		if !p.ok {
			out.vars = append(out.vars, p.expr)
			out.last = i
			continue
		}
		if p.text != "" {
			out.hasConst = true
			if !allSeparators(p.text) {
				out.complex = true
			}
		}
	}
	return out
}

// text returns the path when every variable is empty.
func (c *checker) text(arg ast.Expr) (string, bool) {
	arg = ast.Unparen(arg)
	if text, ok := stringConst(c.pass, arg); ok {
		return text, true
	}
	if call, ok := arg.(*ast.CallExpr); ok {
		if !join(c.pass, call) {
			return "", true
		}
		if call.Ellipsis.IsValid() {
			return "", false
		}
		var elems []string
		for _, expr := range call.Args {
			text, ok := c.text(expr)
			if !ok {
				return "", false
			}
			if text != "" {
				elems = append(elems, text)
			}
		}
		return filepath.Join(elems...), true
	}
	bin, ok := arg.(*ast.BinaryExpr)
	if ok && bin.Op == token.ADD && isString(c.pass, arg) {
		left, ok := c.text(bin.X)
		if !ok {
			return "", false
		}
		right, ok := c.text(bin.Y)
		if !ok {
			return "", false
		}
		return left + right, true
	}
	return "", true
}

// parts returns the constant and variable parts of a path expression, in
// source order.
func (c *checker) parts(arg ast.Expr) ([]part, bool) {
	arg = ast.Unparen(arg)
	if text, ok := stringConst(c.pass, arg); ok {
		return []part{{text: text, ok: true}}, true
	}
	if call, ok := arg.(*ast.CallExpr); ok {
		if !join(c.pass, call) {
			return nil, false
		}
		if call.Ellipsis.IsValid() {
			return nil, false
		}
		var out []part
		for _, expr := range call.Args {
			next, ok := c.parts(expr)
			if !ok {
				return nil, false
			}
			out = append(out, next...)
		}
		return out, true
	}
	bin, ok := arg.(*ast.BinaryExpr)
	if ok && bin.Op == token.ADD && isString(c.pass, arg) {
		var out []part
		for _, expr := range []ast.Expr{bin.X, bin.Y} {
			next, ok := c.parts(expr)
			if !ok {
				return nil, false
			}
			out = append(out, next...)
		}
		return out, true
	}
	return []part{{expr: arg}}, true
}

// join reports whether call is a call of path/filepath.Join or path.Join.
func join(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil {
		return false
	}
	name := fn.FullName()
	return name == "path/filepath.Join" || name == "path.Join"
}

// isString reports whether e has a string type.
func isString(pass *analysis.Pass, e ast.Expr) bool {
	basic, ok := types.Unalias(pass.TypesInfo.TypeOf(e)).(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

// stringConst returns the value of a constant string expression.
func stringConst(pass *analysis.Pass, e ast.Expr) (string, bool) {
	value := pass.TypesInfo.Types[e].Value
	if value == nil || value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(value), true
}

// safe reports whether facts prove path is not the root.
func (c *checker) safe(path path, facts guard) bool {
	if contains(c.pass, facts.nonroot, path.expr) {
		return true
	}
	if len(path.vars) == 0 {
		return false
	}
	last := path.vars[len(path.vars)-1]
	if contains(c.pass, facts.plain, last) {
		if !path.tail {
			return true
		}
		if path.concat && !path.leading && len(path.vars) == 1 {
			if obj := object(c.pass, last); obj != nil {
				if text, ok := c.values[obj]; ok {
					return !rootPath(filepath.Clean(text + path.tailText))
				}
			}
		}
	}
	if path.leading || path.complex || len(path.vars) > 1 {
		return false
	}
	return c.both(facts, last)
}

// both reports whether facts prove expr non-empty and not the root.
func (c *checker) both(facts guard, expr ast.Expr) bool {
	var (
		nonempty = contains(c.pass, facts.nonempty, expr)
		nonroot  = contains(c.pass, facts.nonroot, expr)
	)
	return nonempty && nonroot
}

// factsOf returns the safe facts that cond proves when truth is its value.
func factsOf(pass *analysis.Pass, cond ast.Expr, truth bool) guard {
	cond = ast.Unparen(cond)
	switch cond := cond.(type) {
	case *ast.UnaryExpr:
		if cond.Op == token.NOT {
			return factsOf(pass, cond.X, !truth)
		}
	case *ast.BinaryExpr:
		switch cond.Op {
		case token.LAND:
			if truth {
				return factsOf(pass, cond.X, true).
					merge(factsOf(pass, cond.Y, true))
			}
		case token.LOR:
			if !truth {
				return factsOf(pass, cond.X, false).
					merge(factsOf(pass, cond.Y, false))
			}
		case token.EQL, token.NEQ, token.LSS, token.GTR,
			token.LEQ, token.GEQ:
			return cmpFacts(pass, cond, truth)
		}
	}
	return guard{}
}

// cmpFacts returns the safe facts a comparison proves.
func cmpFacts(pass *analysis.Pass, cmp *ast.BinaryExpr, truth bool) guard {
	if arg, ok := lenArg(pass, cmp.X); ok {
		var (
			n, ok    = intConst(pass, cmp.Y)
			nonempty = ok && stable(pass, arg) &&
				compare(0, cmp.Op, n) != truth
		)
		return lenFacts(arg, nonempty)
	}
	if arg, ok := lenArg(pass, cmp.Y); ok {
		var (
			n, ok    = intConst(pass, cmp.X)
			op       = swap(cmp.Op)
			nonempty = ok && stable(pass, arg) && compare(0, op, n) != truth
		)
		return lenFacts(arg, nonempty)
	}
	if cmp.Op != token.EQL && cmp.Op != token.NEQ {
		return guard{}
	}
	if text, ok := stringConst(pass, cmp.Y); ok {
		return strFacts(pass, cmp.X, text, (cmp.Op == token.NEQ) == truth)
	}
	if text, ok := stringConst(pass, cmp.X); ok {
		return strFacts(pass, cmp.Y, text, (cmp.Op == token.NEQ) == truth)
	}
	return guard{}
}

// strFacts returns the fact that x compares with text when safe is true.
func strFacts(pass *analysis.Pass, x ast.Expr, text string, safe bool) guard {
	if !safe {
		return guard{}
	}
	clean, ok := cleanArg(pass, x)
	if ok {
		if text != "" && isRoot(text) && stable(pass, clean) {
			return guard{nonroot: []ast.Expr{ast.Unparen(clean)}}
		}
		return guard{}
	}
	if text == "" && stable(pass, x) {
		return guard{nonempty: []ast.Expr{ast.Unparen(x)}}
	}
	return guard{}
}

// stable reports whether reading e again gives the same value.
func stable(pass *analysis.Pass, e ast.Expr) bool {
	var effectful bool
	ast.Inspect(e, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && astexpr.Effectful(pass, call) && !pureCall(pass, call) {
			effectful = true
		}
		return !effectful
	})
	return !effectful
}

// lenFacts returns the fact that a length is not zero, when nonempty is true.
func lenFacts(x ast.Expr, nonempty bool) guard {
	if !nonempty {
		return guard{}
	}
	return guard{nonempty: []ast.Expr{ast.Unparen(x)}}
}

// intConst returns the value of a constant integer expression.
func intConst(pass *analysis.Pass, e ast.Expr) (int64, bool) {
	value := pass.TypesInfo.Types[e].Value
	if value == nil || value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(value)
}

// compare evaluates x op y.
func compare(x int64, op token.Token, y int64) bool {
	switch op {
	case token.EQL:
		return x == y
	case token.NEQ:
		return x != y
	case token.LSS:
		return x < y
	case token.GTR:
		return x > y
	case token.LEQ:
		return x <= y
	case token.GEQ:
		return x >= y
	}
	return false
}

// lenArg returns the argument of a len call.
func lenArg(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[ident] != types.Universe.Lookup("len") {
		return nil, false
	}
	return call.Args[0], true
}

// cleanArg returns the argument of a filepath.Clean or path.Clean call.
func cleanArg(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil {
		return nil, false
	}
	switch fn.FullName() {
	case "path/filepath.Clean", "path.Clean":
		return call.Args[0], true
	}
	return nil, false
}

// swap returns the comparison with its operands exchanged.
func swap(op token.Token) token.Token {
	switch op {
	case token.LSS:
		return token.GTR
	case token.GTR:
		return token.LSS
	case token.LEQ:
		return token.GEQ
	case token.GEQ:
		return token.LEQ
	}
	return op
}

// allow returns g with expr known non-empty and not the root.
func (g guard) allow(expr ast.Expr) guard {
	expr = ast.Unparen(expr)
	return guard{
		nonempty: append(slices.Clone(g.nonempty), expr),
		nonroot:  append(slices.Clone(g.nonroot), expr),
		plain:    g.plain,
	}
}

// fixed returns g with expr known to keep the path below the root.
func (g guard) fixed(expr ast.Expr) guard {
	expr = ast.Unparen(expr)
	return guard{
		nonempty: g.nonempty,
		nonroot:  g.nonroot,
		plain:    append(slices.Clone(g.plain), expr),
	}
}

// merge returns the facts of both guards.
func (g guard) merge(other guard) guard {
	return guard{
		nonempty: append(slices.Clone(g.nonempty), other.nonempty...),
		nonroot:  append(slices.Clone(g.nonroot), other.nonroot...),
		plain:    append(slices.Clone(g.plain), other.plain...),
	}
}

// intersect returns the facts both guards hold.
func (g guard) intersect(pass *analysis.Pass, other guard) guard {
	return guard{
		nonempty: common(pass, g.nonempty, other.nonempty),
		nonroot:  common(pass, g.nonroot, other.nonroot),
		plain:    common(pass, g.plain, other.plain),
	}
}

// common returns the expressions that occur in both lists.
func common(pass *analysis.Pass, a, b []ast.Expr) (out []ast.Expr) {
	for _, expr := range a {
		if contains(pass, b, expr) {
			out = append(out, expr)
		}
	}
	return out
}

// keepLocal returns g without facts on a package variable or a field.
func (g guard) keepLocal(pass *analysis.Pass) guard {
	return guard{
		nonempty: localFacts(pass, g.nonempty),
		nonroot:  localFacts(pass, g.nonroot),
		plain:    localFacts(pass, g.plain),
	}
}

// localFacts returns the facts that rest on a local variable.
func localFacts(pass *analysis.Pass, facts []ast.Expr) (out []ast.Expr) {
	for _, fact := range facts {
		if isLocal(pass, fact) {
			out = append(out, fact)
		}
	}
	return out
}

// isLocal reports whether e reads only local variables and literals.
func isLocal(pass *analysis.Pass, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		obj, ok := pass.TypesInfo.ObjectOf(e).(*types.Var)
		return ok && obj.Parent() != pass.Pkg.Scope() && !obj.IsField()
	case *ast.BasicLit:
		return true
	case *ast.BinaryExpr:
		return isLocal(pass, e.X) && isLocal(pass, e.Y)
	case *ast.CallExpr:
		if join(pass, e) {
			for _, arg := range e.Args {
				if !isLocal(pass, arg) {
					return false
				}
			}
			return true
		}
		if arg, ok := cleanArg(pass, e); ok {
			return isLocal(pass, arg)
		}
	}
	return false
}

// without returns g with every fact that mentions one of exprs removed.
func (g guard) without(pass *analysis.Pass, exprs []ast.Expr) guard {
	return guard{
		nonempty: remove(pass, g.nonempty, exprs),
		nonroot:  remove(pass, g.nonroot, exprs),
		plain:    remove(pass, g.plain, exprs),
	}
}

// remove returns facts with every expression that mentions a write removed.
func remove(pass *analysis.Pass, facts, writes []ast.Expr) []ast.Expr {
	out := make([]ast.Expr, 0, len(facts))
	for _, fact := range facts {
		var changed bool
		for _, write := range writes {
			if astexpr.Mentions(pass, fact, write) {
				changed = true
				break
			}
			index, ok := ast.Unparen(write).(*ast.IndexExpr)
			if ok && astexpr.Mentions(pass, fact, index.X) {
				changed = true
				break
			}
		}
		if !changed {
			out = append(out, fact)
		}
	}
	return out
}

// contains reports whether facts hold expr.
func contains(pass *analysis.Pass, facts []ast.Expr, expr ast.Expr) bool {
	for _, fact := range facts {
		if same(pass, fact, expr) {
			return true
		}
	}
	return false
}

// same reports whether a and b read the same value. It extends astexpr.Same
// to operations and calls, whose operands it compares in turn.
func same(pass *analysis.Pass, a, b ast.Expr) bool {
	if astexpr.Same(pass, a, b) {
		return true
	}
	a, b = ast.Unparen(a), ast.Unparen(b)
	switch a := a.(type) {
	case *ast.BinaryExpr:
		bin, ok := b.(*ast.BinaryExpr)
		return ok && a.Op == bin.Op &&
			same(pass, a.X, bin.X) && same(pass, a.Y, bin.Y)
	case *ast.CallExpr:
		call, ok := b.(*ast.CallExpr)
		if !ok {
			return false
		}
		if a.Ellipsis.IsValid() != call.Ellipsis.IsValid() {
			return false
		}
		if len(a.Args) != len(call.Args) || !same(pass, a.Fun, call.Fun) {
			return false
		}
		for i, arg := range a.Args {
			if !same(pass, arg, call.Args[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// written returns the expressions an assignment in node writes, including the
// assignments in a function literal.
func written(node ast.Node) []ast.Expr {
	var out []ast.Expr
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			out = append(out, n.Lhs...)
		case *ast.IncDecStmt:
			out = append(out, n.X)
		case *ast.RangeStmt:
			if n.Key != nil {
				out = append(out, n.Key)
			}
			if n.Value != nil {
				out = append(out, n.Value)
			}
		}
		return true
	})
	return out
}

// addressed returns the expressions whose address node takes or reaches
// through an alias.
func (c *checker) addressed(node ast.Node) []ast.Expr {
	var out []ast.Expr
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				out = append(out, n.X)
			}
		case *ast.Ident:
			if expr, ok := c.aliases[c.pass.TypesInfo.ObjectOf(n)]; ok {
				out = append(out, expr)
			}
		}
		return true
	})
	return out
}

// terminates reports whether stmt prevents the statements after it from
// running on this path.
func (c *checker) terminates(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return stmt.Tok != token.GOTO
	case *ast.BlockStmt:
		return len(stmt.List) > 0 && c.terminates(stmt.List[len(stmt.List)-1])
	case *ast.IfStmt:
		if stmt.Else == nil {
			return false
		}
		return c.terminates(stmt.Body) && c.terminates(stmt.Else)
	case *ast.LabeledStmt:
		return c.terminates(stmt.Stmt)
	case *ast.ExprStmt:
		return isPanic(c.pass, stmt.X)
	}
	return false
}

// isPanic reports whether e is a call to the predeclared panic.
func isPanic(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[ident] == types.Universe.Lookup("panic")
}
