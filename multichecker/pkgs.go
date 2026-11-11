package multichecker

import (
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// roots returns the packages a run should analyze, given the ones a load
// returned. The go command runs a vet tool on the test variant of a package
// and not on the package itself, and cmd/go/internal/vet/vet.go says why:
// running both applies each fix twice and reports each diagnostic twice, and
// running only the package makes a "closed world" rule unsound, because a test
// file can declare what the rule counted. So a package with tests is replaced
// by its test variant, which holds every file of the package plus the test
// files. The generated main of a test binary is dropped, since it has no
// source of its own.
func roots(pkgs []*packages.Package) []*packages.Package {
	byID := make(map[string]*packages.Package, len(pkgs))
	for _, p := range pkgs {
		byID[p.ID] = p
	}
	out := make([]*packages.Package, 0, len(pkgs))
	for _, p := range pkgs {
		if isTestMain(p, byID) {
			continue
		}
		// A variant arrives twice: once by its own ID and once as the
		// replacement for the package it extends. Keep the copy keyed by
		// the base ID, unless nothing is.
		if base, ok := baseOf(p); ok {
			if _, claimed := byID[base]; claimed {
				continue
			}
		}
		if variant, ok := byID[testVariantID(p.ID)]; ok {
			out = append(out, variant)
			continue
		}
		out = append(out, p)
	}
	return out
}

// testVariantID returns the ID go/packages gives the package with its test
// files added, as in "example.com/p [example.com/p.test]".
func testVariantID(id string) string {
	return id + " [" + id + ".test]"
}

// baseOf returns the package a test variant extends, as in the
// "example.com/p" of "example.com/p [example.com/p.test]".
func baseOf(p *packages.Package) (string, bool) {
	base, _, ok := strings.Cut(p.ID, " [")
	if !ok {
		return "", false
	}
	return base, true
}

// isTestMain reports whether p is the generated main of a test binary: the
// main package whose ID ends in .test, in the directory of the package it
// tests. The directory distinguishes it from a main package in a directory
// that ends in .test.
func isTestMain(p *packages.Package, byID map[string]*packages.Package) bool {
	if p.Name != "main" {
		return false
	}
	under, ok := strings.CutSuffix(p.ID, ".test")
	if !ok {
		return false
	}
	base, ok := byID[under]
	return ok && base.Dir == p.Dir
}

// needsFacts reports whether any analyzer produces or consumes facts, which
// needs the syntax of every dependency.
func needsFacts(analyzers []*analysis.Analyzer) bool {
	var (
		seen  = make(map[*analysis.Analyzer]struct{})
		visit func(*analysis.Analyzer) bool
	)
	visit = func(a *analysis.Analyzer) bool {
		if _, ok := seen[a]; ok {
			return false
		}
		seen[a] = struct{}{}
		if a.FactTypes != nil {
			return true
		}
		return slices.ContainsFunc(a.Requires, visit)
	}
	return slices.ContainsFunc(analyzers, visit)
}
