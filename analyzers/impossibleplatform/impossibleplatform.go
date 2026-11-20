// Package impossibleplatform provides an analysis.Analyzer that reports a
// comparison of runtime.GOOS or runtime.GOARCH with a value the file's build
// constraints rule out.
//
// The build-constraint section of the go command's documentation says a file
// whose name matches *_GOOS, *_GOARCH, or *_GOOS_GOARCH is "considered to have
// an implicit build constraint requiring those terms"
// (https://pkg.go.dev/cmd/go#hdr-Build_constraints), and a //go:build line
// before the package clause states more of them. Package runtime declares GOOS
// and GOARCH as constants holding the value its build was given, so a file
// constrained to windows answers runtime.GOOS == "linux" with false in every
// build of that target, and runtime.GOOS != "linux" with true.
//
// The rule reads the //go:build line and the file name, and reads the
// // +build lines of a file that has no //go:build line.
//
// A tag that names no operating system and no architecture is free: a build
// may set it or leave it clear. A comparison that some assignment of the free
// tags lets through is left alone: with -tags=alpha, a file headed
// //go:build alpha || windows also builds where GOOS is linux, so
// runtime.GOOS == "linux" survives. A tag that another value implies holds for
// that value too, since those are the tags the go command sets: android
// satisfies linux, ios satisfies darwin, illumos satisfies solaris, and unix
// holds for every Unix-like GOOS.
//
// A tag can also be set on the command line, so a run of -tags=windows builds
// a file named a_windows.go on a darwin machine, where GOOS is darwin. The
// rule reads a file name the way the go command reads one, and does not know
// the tags a run adds.
//
// The rule reads the files the current build includes. A file the build
// excludes is not offered to the analyzer, so a comparison in it waits for a
// run on a platform that includes it.
//
// The rule reads == and != only, and only a string literal: a comparison with
// an ordering operator or against a named constant is left alone. A literal
// that names no value the go command knows is left alone as well, which covers
// a target added after this rule was written, a fork's target, and a typo.
//
// The fix writes the value the comparison has in the file's builds: false for
// == and true for !=. It is withheld when a comment sits inside the
// comparison, because the edit would delete the comment.
//
// The rule is adapted from staticcheck's SA4032, which reports the same
// comparisons without a fix and reads only the left operand. The README lists
// every check and exemption.
package impossibleplatform

import (
	"go/ast"
	"go/build/constraint"
	"go/constant"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/buildtag"
)

var Analyzer = &analysis.Analyzer{
	Name: "impossibleplatform",
	Doc:  "reports a runtime.GOOS or runtime.GOARCH comparison the file rules out",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		expr := constraints(pass, file)
		ast.Inspect(file, func(n ast.Node) bool {
			cmp, ok := n.(*ast.BinaryExpr)
			if !ok || (cmp.Op != token.EQL && cmp.Op != token.NEQ) {
				return true
			}
			check(pass, file, expr, cmp)
			return true
		})
	}
	return nil, nil
}

// check reports a comparison the file's constraints give one answer.
func check(pass *analysis.Pass, file *ast.File, expr constraint.Expr, cmp *ast.BinaryExpr) {
	name, value, ok := comparison(pass, cmp)
	if !ok || expr == nil {
		return
	}
	targetHolds := targetHolds(name, value)
	if targetHolds == nil {
		return
	}
	if sat, decided := possible(expr, targetHolds); !decided || sat {
		return
	}
	d := analysis.Diagnostic{
		Pos:     cmp.Pos(),
		End:     cmp.End(),
		Message: "the file's build constraints rule out " + name + "=" + value,
	}
	if !commented(file.Comments, cmp.Pos(), cmp.End()) {
		folded := "false"
		if cmp.Op == token.NEQ {
			folded = "true"
		}
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Replace with " + folded,
			TextEdits: []analysis.TextEdit{{
				Pos:     cmp.Pos(),
				End:     cmp.End(),
				NewText: []byte(folded),
			}},
		}}
	}
	pass.Report(d)
}

// constraints returns the condition a build must satisfy to compile the file,
// or nil when it has none.
func constraints(pass *analysis.Pass, file *ast.File) constraint.Expr {
	var (
		name = pass.Fset.File(file.FileStart).Name()
		out  constraint.Expr
	)
	for _, tag := range buildtag.NameTags(name) {
		out = and(out, &constraint.TagExpr{Tag: tag})
	}
	return and(out, header(pass, file))
}

// header returns the condition the comments in the file's header state, or nil
// when they state none.
func header(pass *analysis.Pass, file *ast.File) constraint.Expr {
	cutoff, ok := buildtag.HeaderEnd(pass, file)
	if !ok {
		return nil
	}
	var build, plus constraint.Expr
	for _, group := range file.Comments {
		if group.Pos() >= cutoff {
			break
		}
		for _, c := range group.List {
			switch {
			case constraint.IsGoBuild(c.Text):
				if build == nil {
					build = parse(c.Text)
				}
			case constraint.IsPlusBuild(c.Text):
				plus = and(plus, parse(c.Text))
			}
		}
	}
	if build != nil {
		return build
	}
	return plus
}

// parse returns the condition a build line states, or nil when the line does
// not parse.
func parse(line string) constraint.Expr {
	expr, err := constraint.Parse(line)
	if err != nil {
		return nil
	}
	return expr
}

// and returns the conjunction of two conditions, either of which may be nil.
func and(x, y constraint.Expr) constraint.Expr {
	switch {
	case x == nil:
		return y
	case y == nil:
		return x
	}
	return &constraint.AndExpr{X: x, Y: y}
}

// comparison returns GOOS or GOARCH, whichever the comparison names, and the
// literal it compares against, on either side.
func comparison(pass *analysis.Pass, cmp *ast.BinaryExpr) (name, value string, ok bool) {
	if name = targetOf(pass, cmp.X); name != "" {
		if value, ok = literal(pass, cmp.Y); ok {
			return name, value, true
		}
	}
	if name = targetOf(pass, cmp.Y); name != "" {
		if value, ok = literal(pass, cmp.X); ok {
			return name, value, true
		}
	}
	return "", "", false
}

// targetOf returns GOOS or GOARCH when the expression names one of them.
func targetOf(pass *analysis.Pass, expr ast.Expr) string {
	var ident *ast.Ident
	switch expr := ast.Unparen(expr).(type) {
	case *ast.Ident:
		ident = expr
	case *ast.SelectorExpr:
		ident = expr.Sel
	default:
		return ""
	}
	c, ok := pass.TypesInfo.Uses[ident].(*types.Const)
	if !ok || c.Pkg() == nil || c.Pkg().Path() != "runtime" {
		return ""
	}
	switch c.Name() {
	case "GOOS", "GOARCH":
		return c.Name()
	}
	return ""
}

// literal returns the string a string literal holds.
func literal(pass *analysis.Pass, expr ast.Expr) (string, bool) {
	lit, ok := ast.Unparen(expr).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	tv, ok := pass.TypesInfo.Types[lit]
	if !ok || tv.Value == nil {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// targetHolds returns how a tag evaluates in a build whose GOOS or GOARCH is
// value, and nil when value is not one the go command knows: a target added
// later, a fork's target, and a typo are all left alone.
func targetHolds(name, value string) func(string) (bool, bool) {
	switch name {
	case "GOOS":
		if buildtag.KnownOS[value] {
			return goosHolds(value)
		}
	case "GOARCH":
		if buildtag.KnownArch[value] {
			return archHolds(value)
		}
	}
	return nil
}

// goosHolds returns how a tag evaluates in a build whose GOOS is value. A tag
// the operating system does not decide is left to the build, which is what
// fixed reports.
func goosHolds(value string) func(string) (bool, bool) {
	return func(tag string) (holds, fixed bool) {
		switch {
		case tag == "unix":
			return buildtag.UnixOS[value], true
		case tag == "linux":
			return value == "linux" || value == "android", true
		case tag == "darwin":
			return value == "darwin" || value == "ios", true
		case tag == "solaris":
			return value == "solaris" || value == "illumos", true
		case buildtag.KnownOS[tag]:
			return value == tag, true
		}
		return false, false
	}
}

// archHolds returns how a tag evaluates in a build whose GOARCH is value. A
// tag the architecture does not decide is left to the build, which is what
// fixed reports.
func archHolds(value string) func(string) (bool, bool) {
	return func(tag string) (holds, fixed bool) {
		if buildtag.KnownArch[tag] {
			return value == tag, true
		}
		return false, false
	}
}

// maxFree is the number of free tags the rule assigns every way. The work
// doubles with each one, so a constraint naming more than this is measured as
// unknown rather than tried.
const maxFree = 10

// possible reports whether some build satisfies expr with GOOS or GOARCH set
// to value. A tag the target leaves open is assigned both ways, so a build
// that sets it and a build that leaves it clear are both tried. decided is
// false when the constraint names more free tags than the rule will assign.
func possible(expr constraint.Expr, holds func(string) (bool, bool)) (sat, decided bool) {
	var free []string
	for _, tag := range tags(expr) {
		if _, fixed := holds(tag); !fixed && !slices.Contains(free, tag) {
			free = append(free, tag)
		}
	}
	eval := func(bits int) bool {
		return expr.Eval(func(tag string) bool {
			if holds, fixed := holds(tag); fixed {
				return holds
			}
			return bits&(1<<slices.Index(free, tag)) != 0
		})
	}
	if eval(0) {
		return true, true
	}
	if len(free) > maxFree {
		return false, false
	}
	for bits := 1; bits < 1<<len(free); bits++ {
		if eval(bits) {
			return true, true
		}
	}
	return false, true
}

// tags returns every tag the condition names.
func tags(expr constraint.Expr) []string {
	switch expr := expr.(type) {
	case *constraint.TagExpr:
		return []string{expr.Tag}
	case *constraint.NotExpr:
		return tags(expr.X)
	case *constraint.AndExpr:
		return append(tags(expr.X), tags(expr.Y)...)
	case *constraint.OrExpr:
		return append(tags(expr.X), tags(expr.Y)...)
	}
	return nil
}

// commented reports whether a comment overlaps the span from pos to end.
func commented(groups []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
