// Package packagename provides an analysis.Analyzer that reports a package
// name that gives no sense of its contents, has an underscore, or is not all
// lower case.
//
// The package name prefixes every exported identifier a client uses, so a name
// that gives no sense of the contents is read at every use. The "Package
// names" article says packages named util, common or misc "provide clients
// with no sense of what the package contains".
//
// The report lands on the package name of the first non-generated, non-test
// file in the package.
//
// Adapted from revive's package-naming.
//
// No fix is offered: a new name would break every package that imports this
// one, and the tool cannot choose one.
package packagename

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var (
	Analyzer = &analysis.Analyzer{
		Name: "packagename",
		Doc:  "reports a meaningless package name, a name with an underscore, or a name with a capital letter",
		Run:  run,
	}
	meaningless = map[string]struct{}{
		"common":        {},
		"commons":       {},
		"helper":        {},
		"helpers":       {},
		"misc":          {},
		"miscellaneous": {},
		"util":          {},
		"utilities":     {},
		"utility":       {},
		"utils":         {},
	}
)

func run(pass *analysis.Pass) (any, error) {
	var (
		name = pass.Pkg.Name()
		pos  = namePos(pass)
	)
	if !pos.IsValid() {
		return nil, nil
	}
	if strings.Contains(name, "_") {
		pass.Reportf(pos, "package name %q contains an underscore", name)
		return nil, nil
	}
	if strings.ToLower(name) != name {
		pass.Reportf(pos, "package name %q is not all lower case", name)
		return nil, nil
	}
	if _, ok := meaningless[name]; ok {
		pass.Reportf(pos,
			"package name %q gives no sense of its contents; name the package for what it holds",
			name,
		)
	}
	return nil, nil
}

// namePos returns the position of the package name to report on: the first
// non-generated, non-test file in the package.
//
// A generated file is skipped because the suite drops every diagnostic inside
// one, so a report there would be lost. A test file is skipped because an
// external test package must be named pkg_test, and the author cannot remove
// that underscore. The file name comes from the token.File rather than from
// the position, because a //line directive rewrites positions and leaves the
// file's name alone, so the suffix test reads the name go list gave the file.
func namePos(pass *analysis.Pass) (pos token.Pos) {
	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}
		if strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go") {
			continue
		}
		if !pos.IsValid() {
			pos = file.Name.Pos()
		}
	}
	return pos
}
