package suite

import (
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/ignore"
)

func TestSuiteValidates(t *testing.T) {
	if err := analysis.Validate(ignore.Apply(Analyzers()...)); err != nil {
		t.Fatal(err)
	}
}

// TestListIsSorted keeps the source list in the order a reader expects: adding
// an analyzer in the wrong place is otherwise invisible, because Analyzers
// sorts what it returns.
func TestListIsSorted(t *testing.T) {
	names := make([]string, len(list))
	for i, a := range list {
		names[i] = a.Name
	}
	if !slices.IsSorted(names) {
		t.Fatalf("list is not alphabetical: %v", names)
	}
}

// TestAnalyzersAreSorted is what the catalog test relies on: the documentation
// lists the suite in order, so the suite has an order.
func TestAnalyzersAreSorted(t *testing.T) {
	names := make([]string, 0, len(analyzers))
	for _, a := range analyzers {
		names = append(names, a.Name)
	}
	if !slices.IsSorted(names) {
		t.Fatalf("suite is not alphabetical: %v", names)
	}
}
