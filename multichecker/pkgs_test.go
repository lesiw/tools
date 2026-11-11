package multichecker

import (
	"slices"
	"testing"

	"golang.org/x/tools/go/packages"
)

// ids returns the IDs of the packages roots keeps.
func ids(pkgs []*packages.Package) []string {
	out := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, p.ID)
	}
	return out
}

func TestRootsNoTests(t *testing.T) {
	got, want := ids(roots([]*packages.Package{
		{ID: "example.com/p", Name: "p"},
	})), []string{"example.com/p"}
	if !slices.Equal(got, want) {
		t.Fatalf("roots() = %v", got)
	}
}

func TestRootsTestVariantReplacesPackage(t *testing.T) {
	got, want := ids(roots([]*packages.Package{
		{ID: "example.com/p", Name: "p", Dir: "/p"},
		{ID: "example.com/p.test", Name: "main", Dir: "/p"},
		{ID: "example.com/p [example.com/p.test]", Name: "p", Dir: "/p"},
	})), []string{"example.com/p [example.com/p.test]"}
	if !slices.Equal(got, want) {
		t.Fatalf("roots() = %v", got)
	}
}

func TestRootsKeepsExternalTestPackage(t *testing.T) {
	got, want := ids(roots([]*packages.Package{
		{ID: "example.com/p", Name: "p", Dir: "/p"},
		{ID: "example.com/p.test", Name: "main", Dir: "/p"},
		{ID: "example.com/p [example.com/p.test]", Name: "p", Dir: "/p"},
		{ID: "example.com/p_test [example.com/p.test]", Name: "p_test"},
	})), []string{
		"example.com/p [example.com/p.test]", "example.com/p_test [example.com/p.test]",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("roots() = %v", got)
	}
}

// A main package in a directory that ends in .test is not the generated main
// of a test binary, even when the package its ID trims to is loaded beside it.
func TestRootsDirectoryNamedTest(t *testing.T) {
	got, want := ids(roots([]*packages.Package{
		{ID: "example.com/x", Name: "x", Dir: "/x"},
		{ID: "example.com/x.test", Name: "main", Dir: "/x.test"},
	})), []string{"example.com/x", "example.com/x.test"}
	if !slices.Equal(got, want) {
		t.Fatalf("roots() = %v", got)
	}
}
