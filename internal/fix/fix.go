// Package fix applies the analyzer suite's fixes until the tool has nothing
// left to suggest.
//
// A round applies the fixes, then asks the tool for the patch it would still
// make. The loop stops when that patch is empty. One round is not enough.
// Fixes are independent edits of the same baseline, so two of them can
// conflict. The driver keeps one, and the next round applies the rest.
package fix

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// maxRounds bounds the loop. A tree that does not settle is reported as a
// failure, not run forever.
const maxRounds = 100

// defaultVet is this module's analysis suite.
const defaultVet = "lesiw.io/tools/cmd/vet"

// Main runs the command with args and returns its exit status.
func Main(args []string) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fix:", err)
		return 2
	}
	if err := run(opts.tools()); err != nil {
		fmt.Fprintln(os.Stderr, "fix:", err)
		return 1
	}
	return 0
}

// options is the command line.
type options struct {
	vet     string
	vetArgs []string
}

// parseArgs takes this command's flags out of args. The rest goes to the
// analysis tool, package patterns included.
func parseArgs(args []string) (options, error) {
	opts := options{vet: defaultVet}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			opts.vetArgs = append(opts.vetArgs, arg)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if name != "vettool" {
			opts.vetArgs = append(opts.vetArgs, arg)
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return opts, fmt.Errorf("-%s needs a value", name)
			}
			value = args[i+1]
			i++
		}
		opts.vet = value
	}
	// With no package named, the run covers the current directory, the way
	// `go vet` does.
	named := func(arg string) bool { return !strings.HasPrefix(arg, "-") }
	if !slices.ContainsFunc(opts.vetArgs, named) {
		opts.vetArgs = append(opts.vetArgs, ".")
	}
	return opts, nil
}

// run applies the fixes until the analysis tool has nothing left to change.
func run(apply, pending func() ([]byte, error)) error {
	seen := make(map[[sha256.Size]byte]struct{}, maxRounds)
	for round := range maxRounds {
		if _, err := apply(); err != nil {
			return err
		}
		diff, err := pending()
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(diff)) == 0 {
			fmt.Fprintf(os.Stderr, "fix: settled in %s\n", rounds(round+1))
			return nil
		}
		// The same patch twice means it cannot be applied. Two fixes
		// always conflict, or a file cannot be written.
		sum := sha256.Sum256(diff)
		if _, ok := seen[sum]; ok {
			return fmt.Errorf("the same patch is still pending after %s: "+
				"two fixes conflict, or a file cannot be written",
				rounds(round+1),
			)
		}
		seen[sum] = struct{}{}
		fmt.Fprintf(os.Stderr, "fix: round %d: fixes remain\n", round+1)
	}
	return fmt.Errorf("fixes did not settle in %d rounds", maxRounds)
}

// tools returns the run's two calls into the analysis tool. One applies the
// fixes. The other asks for the patch it would still make.
func (o options) tools() (apply, pending func() ([]byte, error)) {
	call := func(diff bool) func() ([]byte, error) {
		return func() ([]byte, error) {
			args := []string{"-fix"}
			if diff {
				args = append(args, "-diff")
			}
			args = append(args, o.vetArgs...)
			argv, err := toolCommand(o.vet)
			if err != nil {
				return nil, err
			}
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(argv[0], append(argv[1:], args...)...)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				msg := stderr.String()
				// The driver exits non-zero when it sets a fix
				// aside for the next round. A run that set
				// every fix aside left no patch, so there is
				// nothing to apply. Saying it settled would be
				// a claim about the tree that is not true.
				if skippedFixes(msg) {
					fmt.Fprint(os.Stderr, msg)
					if diff && len(bytes.TrimSpace(stdout.Bytes())) == 0 {
						return nil, fmt.Errorf("%s skipped every fix and left no patch", o.vet)
					}
					return stdout.Bytes(), nil
				}
				if msg := strings.TrimSpace(msg); msg != "" {
					return nil, fmt.Errorf("analyze with %s: %w: %s",
						o.vet, err, msg,
					)
				}
				return nil, fmt.Errorf("analyze with %s: %w", o.vet, err)
			}
			return stdout.Bytes(), nil
		}
	}
	return call(false), call(true)
}

// toolCommand returns the command line for name. It tries the go command's
// tool first, because that builds what a module names. Then it tries a program
// on PATH. Last, it tries the package at its latest version, so a tree that
// has added nothing still works.
func toolCommand(name string) ([]string, error) {
	var (
		stderr bytes.Buffer
		probe  = exec.Command("go", "tool", "-n", name)
	)
	probe.Stderr = &stderr
	if err := probe.Run(); err == nil {
		return []string{"go", "tool", name}, nil
	} else if !strings.Contains(stderr.String(), "no such tool") {
		return nil, fmt.Errorf("%s: %s", name,
			strings.TrimSpace(stderr.String()),
		)
	}
	if path, err := exec.LookPath(name); err == nil {
		return []string{path}, nil
	}
	if strings.Contains(name, "/") {
		return []string{"go", "run", name + "@latest"}, nil
	}
	return nil, fmt.Errorf("%s: not a tool of this module, and not on PATH", name)
}

// skippedFixes reports whether msg says the driver set a fix aside for the
// next round. The exit status is non-zero for that and for a hard error alike.
// The message is the only signal.
func skippedFixes(msg string) bool {
	return strings.Contains(msg, "Re-run the command to apply more.") ||
		strings.Contains(msg, "skipped (e.g. due to conflicts)")
}

func rounds(n int) string {
	if n == 1 {
		return "1 round"
	}
	return fmt.Sprintf("%d rounds", n)
}
