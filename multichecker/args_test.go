package multichecker

import (
	"slices"
	"testing"

	"lesiw.io/tools/internal/suite"
)

// buildFlagCases are the command lines the parser has to split between the
// load and the packages.
var buildFlagCases = []struct {
	name     string
	args     []string
	patterns []string
	build    []string
}{{
	name:     "separate value",
	args:     []string{"-tags", "netgo", "./..."},
	patterns: []string{"./..."},
	build:    []string{"-tags", "netgo"},
}, {
	name:     "joined value",
	args:     []string{"-tags=netgo", "./..."},
	patterns: []string{"./..."},
	build:    []string{"-tags=netgo"},
}, {
	name:     "boolean build flag",
	args:     []string{"-trimpath", "."},
	patterns: []string{"."},
	build:    []string{"-trimpath"},
}}

func TestParseArgsBuildFlags(t *testing.T) {
	for _, tt := range buildFlagCases {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseArgs(tt.args, suite.Analyzers())
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(opts.patterns, tt.patterns) {
				t.Errorf("patterns = %q, want %q", opts.patterns, tt.patterns)
			}
			if !slices.Equal(opts.build, tt.build) {
				t.Errorf("build = %q, want %q", opts.build, tt.build)
			}
		})
	}
}

func TestParseArgsRejectsVettool(t *testing.T) {
	args := []string{"-vettool=x", "."}
	if _, err := parseArgs(args, suite.Analyzers()); err == nil {
		t.Fatal("-vettool was accepted")
	}
}

// TestParseArgsAnalyzerFlagKeepsSuite checks that setting one analyzer's
// flag does not narrow the run: a flag names a setting, and only a bare
// analyzer name selects one.
func TestParseArgsAnalyzerFlagKeepsSuite(t *testing.T) {
	args := []string{"-linelen.len=100", "."}
	opts, err := parseArgs(args, suite.Analyzers())
	if err != nil {
		t.Fatal(err)
	}
	if want := len(suite.Analyzers()); len(opts.analyzers) != want {
		t.Fatalf("analyzers = %d, want %d", len(opts.analyzers), want)
	}
}
