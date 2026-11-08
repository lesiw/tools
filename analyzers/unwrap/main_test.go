package unwrap

import (
	"os"
	"testing"

	"lesiw.io/tools/analyzers/linelen"
)

func TestMain(m *testing.M) {
	// The commands set the width they run with; the tests run what the
	// commands run.
	linelen.Config.Len = 79
	os.Exit(m.Run())
}

// The rules have their own golden fixtures and property tests; the analyzer
// wiring is checked separately in unwrap_test.go.
