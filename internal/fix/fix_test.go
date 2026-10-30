package fix

import (
	"slices"
	"strings"
	"testing"
)

type argCase struct {
	name    string
	args    []string
	vet     string
	vetArgs []string
}

var argCases = []argCase{{
	name:    "packages only",
	args:    []string{"./..."},
	vet:     defaultVet,
	vetArgs: []string{"./..."},
}, {
	name:    "analysis tool as two words",
	args:    []string{"-vettool", "x/y", "./..."},
	vet:     "x/y",
	vetArgs: []string{"./..."},
}, {
	name:    "analysis tool as one word",
	args:    []string{"-vettool=x/y", "./..."},
	vet:     "x/y",
	vetArgs: []string{"./..."},
}, {
	name:    "analysis tool with two hyphens",
	args:    []string{"--vettool=x/y", "./..."},
	vet:     "x/y",
	vetArgs: []string{"./..."},
}, {
	name:    "build flag passed on",
	args:    []string{"-tags=netgo", "./..."},
	vet:     defaultVet,
	vetArgs: []string{"-tags=netgo", "./..."},
}}

func TestParseArgs(t *testing.T) {
	for _, tt := range argCases {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseArgs(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if opts.vet != tt.vet {
				t.Errorf("vet = %q, want %q", opts.vet, tt.vet)
			}
			if !slices.Equal(opts.vetArgs, tt.vetArgs) {
				t.Errorf("vetArgs = %q, want %q", opts.vetArgs, tt.vetArgs)
			}
		})
	}
}

func TestParseArgsError(t *testing.T) {
	if _, err := parseArgs([]string{"-vettool"}); err == nil {
		t.Error("parseArgs accepted -vettool with no value")
	}
}

func TestSkippedFixes(t *testing.T) {
	cases := map[string]bool{
		"checker: applied 3 of 5 fixes; 2 files updated. " +
			"(Re-run the command to apply more.)": true,
		"checker: 2 of 5 fixes skipped (e.g. due to conflicts)": true,
		"checker: applied 0 of 1 fix; 0 files updated. " +
			"(Re-run the command to apply more.)": true,
		"checker: open x.go: permission denied":  false,
		"checker: layout: layout did not settle": false,
	}
	for msg, want := range cases {
		if got := skippedFixes(msg); got != want {
			t.Errorf("skippedFixes(%q) = %v, want %v", msg, got, want)
		}
	}
}

func TestRounds(t *testing.T) {
	for n, want := range map[int]string{1: "1 round", 2: "2 rounds"} {
		if got := rounds(n); got != want {
			t.Errorf("rounds(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestToolCommand holds tool resolution to the order a run tries: the go
// command's tool, then PATH, then the package at its latest version.
func TestToolCommand(t *testing.T) {
	// A tool of this module resolves through the go command.
	argv, err := toolCommand(defaultVet)
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 3 || argv[0] != "go" || argv[1] != "tool" {
		t.Errorf("toolCommand(%q) = %q", defaultVet, argv)
	}
	// A program on PATH resolves to its path.
	if argv, err := toolCommand("go"); err != nil {
		t.Fatal(err)
	} else if len(argv) != 1 {
		t.Errorf("toolCommand(go) = %q", argv)
	}
	// A package that is neither falls back to its latest version.
	argv, err = toolCommand("example.com/nothing/cmd/nothing")
	if err != nil {
		t.Fatal(err)
	}
	want := "example.com/nothing/cmd/nothing@latest"
	if argv[1] != "run" || argv[2] != want {
		t.Errorf("toolCommand = %q, want go run %s", argv, want)
	}
}

// runCase is one run: the rounds the tool reports work for, and the error the
// run ends with.
type runCase struct {
	name   string
	rounds int
	stuck  bool
	want   string
}

var runCases = []runCase{{name: "settles"}, {
	name: "applies the rest next round", rounds: 2,
}, {name: "refuses a patch it has seen", stuck: true,
	want: "the same patch is still pending"}}

// TestRun holds the loop to its three exits. A tree settles. A tree needs
// another round. A patch has been seen before.
func TestRun(t *testing.T) {
	for _, tt := range runCases {
		t.Run(tt.name, func(t *testing.T) {
			var (
				rounds = tt.rounds
				stuck  = tt.stuck
				apply  = func() ([]byte, error) {
					if rounds > 0 {
						rounds--
					}
					return nil, nil
				}
				pending = func() ([]byte, error) {
					if stuck || rounds > 0 {
						return []byte("--- a\n+++ b\n"), nil
					}
					return nil, nil
				}
			)
			err := run(apply, pending)
			switch {
			case tt.want == "" && err != nil:
				t.Fatalf("run = %v, want nil", err)
			case tt.want != "" && err == nil:
				t.Fatalf("run = nil, want %q", tt.want)
			case tt.want != "" && !strings.Contains(err.Error(), tt.want):
				t.Fatalf("run = %v, want %q", err, tt.want)
			}
		})
	}
}
