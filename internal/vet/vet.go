// Package vet runs the lesiw.io analyzer suite over Go packages.
package vet

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/tools/go/analysis/multichecker"
)

// Main runs the suite over the packages named in args. It does not return.
//
// The go command drives a vet tool: it loads the packages, chooses the
// test-augmented variant of each, caches the results, and calls the tool back
// with one package at a time. So this command hands the work to
// go vet -vettool, and only analyzes in place when the go command is the one
// that started it. Everything else - the package patterns, -C, -tags, and one
// flag per analyzer - is the go command's business, which is what makes those
// flags behave exactly as they do for go vet itself.
func Main(args []string) {
	if isTool(args) {
		os.Args = append([]string{"vet"}, args...)
		multichecker.Main(filter(analyzers...)...)
		return
	}
	os.Exit(run(args))
}

// isTool reports whether the go command started this program as its vet tool.
//
// The protocol is unitchecker's, which is the prior art for this question:
//
//	-V=full         describe executable               (to the build tool)
//	-flags          describe flags                    (to the build tool)
//	foo.cfg         description of compilation unit (from the build tool)
func isTool(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if len(args) == 1 {
		switch {
		case args[0] == "-flags":
			return true
		case strings.HasPrefix(args[0], "-V="):
			return true
		}
	}
	return strings.HasSuffix(args[len(args)-1], ".cfg")
}

// run asks the go command to vet the named packages with this program as the
// tool, and returns the status the run earned.
func run(args []string) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "vet:", err)
		return 1
	}
	// -C leads a go command line, so it goes ahead of the verb.
	argv := []string{"vet"}
	if len(args) >= 2 && args[0] == "-C" {
		argv = []string{"-C", args[1], "vet"}
		args = args[2:]
	}
	argv = append(argv, "-vettool="+self)
	argv = append(argv, args...)
	cmd := exec.Command("go", argv...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			// A signalled child reports -1, and os.Exit(-1) is 255.
			if code := exit.ExitCode(); code > 0 {
				return code
			}
			return 1
		}
		fmt.Fprintln(os.Stderr, "vet:", err)
		return 1
	}
	return 0
}
