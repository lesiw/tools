package main

import (
	"os"
	"testing"

	"lesiw.io/tools/analyzers/linelen"
)

func TestMain(m *testing.M) {
	// The command sets the width it runs with; the tests run what the
	// command runs.
	linelen.Config.Len = 79
	os.Exit(m.Run())
}
