package suite

import (
	"testing"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/ignore"
)

func TestSuiteValidates(t *testing.T) {
	if err := analysis.Validate(ignore.Apply(Analyzers()...)); err != nil {
		t.Fatal(err)
	}
}
