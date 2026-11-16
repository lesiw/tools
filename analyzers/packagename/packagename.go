// Package packagename provides an analysis.Analyzer that reports a package
// name that gives no sense of its contents, collides with a common standard
// library package, is pkg in a directory with another base name, has an
// underscore, or is not all lower case.
//
// The package name prefixes every exported identifier a client uses, so a name
// that gives no sense of the contents is read at every use. The "Package
// names" article says packages named util, common or misc "provide clients
// with no sense of what the package contains", and that a package should not
// take the name of a popular standard package such as io or http.
//
// A package named pkg whose directory has another base name is reported: the
// Package names article gives the convention that the last element of the
// package path is the package name, so the directory already names the
// package.
//
// The report lands on the package name of the first non-generated, non-test
// file in the package.
//
// The collision check leaves alone the standard library package itself. It
// also leaves alone a package under an internal or vendor directory, which
// clients outside its module cannot import.
//
// The rule does not report api, types, and interfaces, which name a subject a
// package holds.
//
// Leaves alone interface and type: both are Go keywords, so the parser rejects
// them as package names.
//
// Adapted from revive's package-naming.
//
// No fix is offered: a new name would break every package that imports this
// one, and the tool cannot choose one.
package packagename

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var (
	Analyzer = &analysis.Analyzer{
		Name: "packagename",
		Doc:  "reports a meaningless package name, a name that collides with a standard library package, a package named pkg whose directory has another base name, a name with an underscore, or a name with a capital letter",
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

	// commonStd is revive's default list of common standard library packages,
	// keyed by package name and valued by import path.
	commonStd = map[string]string{
		"bufio":    "bufio",
		"bytes":    "bytes",
		"context":  "context",
		"errors":   "errors",
		"filepath": "path/filepath",
		"flag":     "flag",
		"fmt":      "fmt",
		"http":     "net/http",
		"io":       "io",
		"ioutil":   "io/ioutil",
		"json":     "encoding/json",
		"log":      "log",
		"math":     "math",
		"net":      "net",
		"os":       "os",
		"reflect":  "reflect",
		"regexp":   "regexp",
		"runtime":  "runtime",
		"sort":     "sort",
		"strconv":  "strconv",
		"strings":  "strings",
		"sync":     "sync",
		"time":     "time",
		"url":      "net/url",
	}
)

func run(pass *analysis.Pass) (any, error) {
	var (
		name       = pass.Pkg.Name()
		importPath = pass.Pkg.Path()
		pos        = namePos(pass)
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
		return nil, nil
	}
	if name == "pkg" {
		dir := filepath.Base(filepath.Dir(pass.Fset.File(pos).Name()))
		if dir != name {
			pass.Reportf(pos,
				"package name %q does not match its directory %q; the directory already names the package",
				name, dir,
			)
			return nil, nil
		}
	}
	if std := standardCollision(name, importPath); std != "" {
		pass.Reportf(pos,
			"package name %q collides with standard library package path %q; choose a name that distinguishes this package",
			name, std,
		)
	}
	return nil, nil
}

// standardCollision returns the standard library package path that name
// collides with. The standard library package itself is left alone, as is a
// package under an internal or vendor directory, which clients outside its
// module cannot import.
func standardCollision(name, importPath string) string {
	std, ok := commonStd[name]
	if !ok || strings.TrimPrefix(importPath, "std/") == std {
		return ""
	}
	for segment := range strings.SplitSeq(importPath, "/") {
		if segment == "internal" || segment == "vendor" {
			return ""
		}
	}
	return std
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
