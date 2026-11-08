// Package tool runs a driver the way the go command expects to call one.
package tool

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Suite is this module's analysis suite.
const Suite = "lesiw.io/tools/cmd/vet"

// Main calls analyze when the go command started this program as its vet
// tool, and otherwise asks the go command to vet the named packages with this
// program as the tool. It does not return.
//
// The go command drives a vet tool: it loads the packages, chooses the
// test-augmented variant of each, caches the results, and calls the tool back
// with one package at a time. So a command hands the work to go vet -vettool,
// and only analyzes in place when the go command is the one that started it.
// Everything else - the package patterns, -C, -tags, and one flag per analyzer
// - is the go command's business, which is what makes those flags behave
// exactly as they do for go vet itself.
func Main(analyze func()) {
	args := os.Args[1:]
	if isTool(args) {
		os.Args = append([]string{"vet"}, args...)
		analyze()
		return
	}
	os.Exit(run(args))
}

// Resolve returns the path of the program the go command should run as its
// vet tool, and a cleanup for a copy it built itself.
func Resolve(name string) (path string, cleanup func(), err error) {
	cleanup = func() {}
	// A path, as opposed to a package path, starts at the root or the
	// current directory.
	if strings.HasPrefix(name, ".") || filepath.IsAbs(name) {
		abs, err := filepath.Abs(name)
		return abs, cleanup, err
	}
	// A module that names the suite builds it, and the go command says
	// where. That is the common case inside a lesiw module.
	probe := exec.Command("go", "tool", "-n", name)
	if out, err := probe.Output(); err == nil {
		return strings.TrimSpace(string(out)), cleanup, nil
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, cleanup, nil
	}
	if !strings.Contains(name, "/") {
		return "", cleanup, fmt.Errorf("%s: not a tool of this module, and not on PATH", name)
	}
	// Last, install the published package into a directory of our own. A
	// tree that has added nothing still gets a suite.
	dir, err := os.MkdirTemp("", "fix")
	if err != nil {
		return "", cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	var stderr bytes.Buffer
	cmd := exec.Command("go", "install", name+"@latest")
	cmd.Env = append(os.Environ(), "GOBIN="+dir)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", cleanup, fmt.Errorf("install %s: %w: %s", name, err,
			strings.TrimSpace(stderr.String()),
		)
	}
	return filepath.Join(dir, filepath.Base(name)), cleanup, nil
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
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
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
