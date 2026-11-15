package multichecker

import (
	"errors"
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/suite"
)

// helpAnalyzers is one analyzer, so a help request prints a short list.
var helpAnalyzers = []*analysis.Analyzer{{Name: "a", Doc: "reports a thing"}}

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

func TestParseArgsHelpSubcommand(t *testing.T) {
	_, err := parseArgs([]string{"help"}, helpAnalyzers)
	if !errors.Is(err, errHelp) {
		t.Fatalf("parseArgs(help) = %v, want errHelp", err)
	}
	opts, err := parseArgs([]string{"-tags", "netgo", "help"}, helpAnalyzers)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"help"}; !slices.Equal(opts.patterns, want) {
		t.Errorf("patterns = %q, want %q", opts.patterns, want)
	}
}

func TestWithoutFix(t *testing.T) {
	cases := []struct {
		args  []string
		want  []string
		asked bool
	}{{
		[]string{"-fix", "help"}, []string{"help"}, true,
	}, {
		[]string{"--fix", "help"}, []string{"help"}, true,
	}, {
		[]string{"-fix=true", "help"}, []string{"help"}, true,
	}, {
		[]string{"-fix=false", "help"}, []string{"help"}, false,
	}, {
		[]string{"help"}, []string{"help"}, false,
	}}
	for _, tt := range cases {
		rest, asked := withoutFix(tt.args)
		if !slices.Equal(rest, tt.want) || asked != tt.asked {
			t.Errorf(
				"withoutFix(%q) = %q, %v; want %q, %v",
				tt.args, rest, asked, tt.want, tt.asked,
			)
		}
	}
}

// TestHelpText checks that the help output names each analyzer and its Doc.
func TestHelpText(t *testing.T) {
	got, want := helpText(helpAnalyzers), "a: reports a thing\n"
	if got != want {
		t.Errorf("helpText = %q, want %q", got, want)
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
