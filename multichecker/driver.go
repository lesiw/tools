// Package multichecker runs a suite of analyzers as one command.
//
// It supports the two ways a command of this kind is run. The go command
// drives it under go vet -vettool and go fix -fixtool, handing over one
// package at a time as a configuration file; the unitchecker does that work,
// and the go command decides which packages exist, including its choice to
// run the test variant rather than the package itself.
//
// Run directly, it loads the packages itself and makes the same choice: the
// test variant of a package stands in for the package itself.
package multichecker

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/analysis/unitchecker"

	"lesiw.io/tools/internal/ignore"
)

// isHelp reports whether the command line asks for the analyzer list. The
// subcommand is only read from the first argument.
func isHelp(args []string) bool {
	return len(args) > 0 && args[0] == "help"
}

// Exit codes. 1 means the run could not analyze what it was given, 2 means the
// command line could not be acted on, and 3 means the run produced
// diagnostics.
const (
	exitOK      = 0
	exitFailed  = 1
	exitUsage   = 2
	exitFinding = 3
)

// Main reports the suite's diagnostics over the packages named on the command
// line, and applies the fixes instead when the command line asks for them. It
// does not return.
func Main(analyzers ...*analysis.Analyzer) {
	args := os.Args[1:]
	if drivenByGo(args) {
		unitchecker.Main(ignore.Apply(analyzers...)...)
		return
	}
	if isHelp(args) {
		help(analyzers)
		os.Exit(exitOK)
	}
	if rest, ok := withoutFix(args); ok {
		os.Exit(fixRun(rest, analyzers))
	}
	os.Exit(vet(analyzers, args))
}

// Fix applies the suite's fixes, repeating until no fix is left. It does not
// return.
func Fix(analyzers ...*analysis.Analyzer) {
	args := os.Args[1:]
	if drivenByGo(args) {
		unitchecker.Main(ignore.Apply(analyzers...)...)
		return
	}
	if isHelp(args) {
		help(analyzers)
		os.Exit(exitOK)
	}
	os.Exit(fixRun(args, analyzers))
}

// drivenByGo reports whether the go command is running this binary as a vet
// tool. It asks for the version and the flag list first, then runs the tool
// once per package with a .cfg file holding that package's files and flags.
func drivenByGo(args []string) bool {
	for _, arg := range args {
		switch {
		case arg == "-flags", arg == "-V", strings.HasPrefix(arg, "-V="):
			return true
		case strings.HasSuffix(arg, ".cfg"):
			return true
		}
	}
	return false
}

// withoutFix reports whether args ask for fixes, and returns them without the
// flag.
func withoutFix(args []string) ([]string, bool) {
	var (
		out   = make([]string, 0, len(args))
		asked bool
	)
	for _, arg := range args {
		switch arg {
		case "-fix", "--fix", "-fix=true":
			asked = true
		case "-fix=false":
		default:
			out = append(out, arg)
		}
	}
	return out, asked
}

// vet analyzes the named packages and prints what the suite reports.
func vet(analyzers []*analysis.Analyzer, args []string) int {
	opts, err := parseArgs(args, analyzers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vet: %v\n", err)
		return exitUsage
	}
	loaded, bad, err := load(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vet: %v\n", err)
		return exitUsage
	}
	graph, err := checker.Analyze(ignore.Apply(opts.analyzers...), loaded, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vet: %v\n", err)
		return exitFailed
	}
	// A negative context count prints the diagnostic alone, which is the
	// shape go vet prints.
	if err := graph.PrintText(os.Stderr, -1); err != nil {
		fmt.Fprintf(os.Stderr, "vet: %v\n", err)
		return exitFailed
	}
	failed := bad > 0
	var found bool
	for _, act := range graph.Roots {
		if act.Err != nil {
			failed = true
		}
		if len(act.Diagnostics) > 0 {
			found = true
		}
	}
	switch {
	case failed:
		return exitFailed
	case found:
		return exitFinding
	}
	return exitOK
}
