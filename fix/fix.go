// Package fix applies the analyzer suite's fixes until the suite has nothing
// left to suggest.
//
// The go command is the driver. A round asks it which analyzers still have
// fixes, then runs one analyzer at a time with -fix, so two analyzers never
// edit the same file in one pass. That matters twice over: the go command
// keeps the patch of a tool that exits zero, and a tool that sets a fix aside
// exits non-zero, so a single-analyzer run is the one shape whose work is
// never discarded.
package fix

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"

	"lesiw.io/tools/internal/tool"
)

// maxRounds bounds the loop. A tree that does not settle is reported as a
// failure, not run forever.
const maxRounds = 100

// errUsage marks a command line this command cannot act on.
var errUsage = fmt.Errorf("usage: fix [-diff] [-vettool prog] [go vet flags] [packages]")

// Main runs the command with args and returns its exit status. The exit
// status is the only thing decided here; the command itself is run.
func Main(args []string) int {
	err := run(args)
	if err == nil {
		return 0
	}
	if msg := err.Error(); msg != "" {
		fmt.Fprintln(os.Stderr, "fix:", msg)
	}
	if errors.Is(err, errUsage) {
		return 2
	}
	return 1
}

// run is the command. A defer here is scoped to the whole run, which is why
// the work lives here rather than in Main.
func run(args []string) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	toolPath, cleanup, err := tool.Resolve(opts.vet)
	if err != nil {
		return err
	}
	defer cleanup()
	if opts.diff {
		return report(opts, toolPath)
	}
	return loop(opts, toolPath)
}

// report prints the fixes that are pending and fails when there are any. It
// writes nothing, so a check leaves the tree as it found it.
//
// A tree with nothing pending is settled: a fix appears only because an
// earlier one changed the code, so an empty first report is an empty run.
func report(o options, tool string) error {
	return reportWith(o, tool)
}

// reportWith is report over any driver, so the verdict is testable.
func reportWith(d driver, tool string) error {
	pending, err := d.report(tool)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	for _, line := range pending {
		fmt.Fprintln(os.Stderr, line)
	}
	return fmt.Errorf("%d fixes pending", len(pending))
}

// options is the command line: the suite to run, whether to write, and
// everything that goes to the go command, package patterns included.
type options struct {
	vet  string
	diff bool
	args []string
}

// parseArgs takes this command's own flag out of args. The rest goes to the
// go command, so an analyzer flag means there what it means there: -NAME runs
// one analyzer, -NAME=false turns one off, and -NAME.flag=value sets one of
// its own.
func parseArgs(args []string) (options, error) {
	opts := options{vet: tool.Suite}
	for i := 0; i < len(args); i++ {
		var (
			arg                   = args[i]
			name, value, hasValue = strings.Cut(
				strings.TrimLeft(arg, "-"), "=",
			)
		)
		if strings.HasPrefix(arg, "-") && name == "diff" && !hasValue {
			opts.diff = true
			continue
		}
		if !strings.HasPrefix(arg, "-") || name != "vettool" {
			opts.args = append(opts.args, arg)
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return opts, fmt.Errorf("%w: -%s needs a value",
					errUsage, name,
				)
			}
			value = args[i+1]
			i++
		}
		opts.vet = value
	}
	// With no package named, the run covers the current directory, the way go
	// vet does.
	named := func(arg string) bool { return !strings.HasPrefix(arg, "-") }
	if !slices.ContainsFunc(opts.args, named) {
		opts.args = append(opts.args, ".")
	}
	return opts, nil
}

// Tool returns the analyzer suite this command would run, building or
// installing it the way a fix run would.
func Tool() (string, func(), error) { return tool.Resolve(tool.Suite) }

// driver is the two things a round asks of the go command.
type driver interface {
	report(tool string) ([]string, error)
	apply(tool, name string) error
}

// loop applies fixes one analyzer at a time until a report comes back empty.
//
// The go command discards the patch for a whole package when one of its fixes
// is skipped. Packages with no skipped fix still get theirs, so the loop
// continues, and a round whose report repeats fails with the findings that
// remain.
func loop(d driver, tool string) error {
	seen := make(map[[sha256.Size]byte]struct{}, maxRounds)
	for round := 1; round <= maxRounds; round++ {
		pending, err := d.report(tool)
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			if round == 1 {
				fmt.Fprintln(os.Stderr, "fix: nothing to fix")
				return nil
			}
			fmt.Fprintf(os.Stderr, "fix: settled in %s\n", rounds(round-1))
			return nil
		}
		// The same report twice means no fix in it can be applied. Two
		// diagnostics of one analyzer overlap, or a file cannot be
		// written.
		sum := sha256.Sum256([]byte(strings.Join(pending, "\n")))
		if _, ok := seen[sum]; ok {
			return fmt.Errorf(
				"the same fixes are still pending after %s: %s",
				rounds(round-1), strings.Join(head(pending, 3), "; "),
			)
		}
		seen[sum] = struct{}{}
		for _, name := range analyzers(pending) {
			err := d.apply(tool, name)
			if err == nil || skipped(err) {
				continue
			}
			return err
		}
		fmt.Fprintf(os.Stderr, "fix: round %d: %s\n", round,
			strings.Join(analyzers(pending), ", "),
		)
	}
	return fmt.Errorf("fixes did not settle in %d rounds", maxRounds)
}

// skipped reports whether err says the tool set a fix aside. The exit status
// is non-zero for that and for a hard error alike; the message is the only
// signal.
func skipped(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Re-run the command to apply more.") ||
		strings.Contains(msg, "skipped (e.g. due to conflicts)")
}

// head returns at most n of a report's lines.
func head(lines []string, n int) []string {
	if len(lines) > n {
		return append(lines[:n:n], fmt.Sprintf("and %d more", len(lines)-n))
	}
	return lines
}

// analyzers returns the distinct analyzer names in a report.
func analyzers(pending []string) (names []string) {
	for _, line := range pending {
		name, _, _ := strings.Cut(line, "\t")
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// report asks the go command which diagnostics still carry a fix. Each line
// of the result is an analyzer name and a description of the fix.
func (o options) report(tool string) ([]string, error) {
	var (
		stdout, stderr bytes.Buffer
		cmd            = o.goCommand("vet", "-vettool="+tool, "-json")
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Diagnostics make the go command exit non-zero, and the report is
	// what this command came for, so the status is not the answer.
	_ = cmd.Run()
	pending, err := parseReport(&stdout)
	if len(pending) == 0 && err != nil && stderr.Len() > 0 {
		// Nothing parsed and the go command had something to say: the
		// run failed rather than coming back clean.
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("report with %s: %s", o.vet, msg)
		}
	}
	return pending, nil
}

// parseReport reads the go command's JSON report and returns one line per
// diagnostic that carries a fix, naming the analyzer that offered it.
func parseReport(r io.Reader) ([]string, error) {
	var (
		pending []string
		dec     = json.NewDecoder(r)
	)
	for {
		var pkgs map[string]map[string][]struct {
			Posn  string `json:"posn"`
			Fixes []struct {
				Message string `json:"message"`
			} `json:"suggested_fixes"`
		}
		if err := dec.Decode(&pkgs); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		for _, byAnalyzer := range pkgs {
			for name, diags := range byAnalyzer {
				for _, d := range diags {
					if len(d.Fixes) == 0 {
						continue
					}
					pending = append(pending, fmt.Sprintf("%s\t%s: %s",
						name, d.Posn, d.Fixes[0].Message,
					),
					)
				}
			}
		}
	}
	slices.Sort(pending)
	return slices.Compact(pending), nil
}

// apply runs one analyzer's fixes.
func (o options) apply(tool, name string) error {
	var (
		stderr bytes.Buffer
		cmd    = o.goCommand("vet", "-vettool="+tool, "-fix", "-"+name)
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fix with %s: %w: %s", name, err,
			strings.TrimSpace(stderr.String()),
		)
	}
	return nil
}

// goCommand returns a go command line with args appended to the verb. A -C
// has to lead the go command line, so it is lifted out of the arguments.
func (o options) goCommand(verb string, args ...string) *exec.Cmd {
	var (
		argv = []string{verb}
		rest = o.args
	)
	if len(rest) >= 2 && rest[0] == "-C" {
		argv = []string{"-C", rest[1], verb}
		rest = rest[2:]
	}
	argv = append(argv, args...)
	argv = append(argv, rest...)
	return exec.Command("go", argv...)
}

func rounds(n int) string {
	if n == 1 {
		return "1 round"
	}
	return fmt.Sprintf("%d rounds", n)
}
