package multichecker

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"

	"lesiw.io/tools/internal/ignore"
)

// maxRounds bounds the loop. A tree that does not settle is reported as a
// failure, not run forever.
const maxRounds = 100

// errUsage marks a command line this command cannot act on.
var errUsage = fmt.Errorf("usage: [build flags] [analyzer flags] [packages]")

// unsupported are the flags this command does not take: go vet's output and
// tool flags, the profiler's, and the help flags.
var unsupported = map[string]struct{}{
	"h": {}, "help": {}, "json": {}, "c": {}, "diff": {},
	"vettool": {}, "fix": {}, "debug": {}, "cpuprofile": {},
	"memprofile": {}, "trace": {},
}

// buildValueFlags are the go build flags that take their value as the next
// argument. A flag in this set and its value belong to the load together.
var buildValueFlags = map[string]struct{}{
	"asmflags":      {},
	"buildmode":     {},
	"compiler":      {},
	"covermode":     {},
	"coverpkg":      {},
	"gccgoflags":    {},
	"gcflags":       {},
	"installsuffix": {},
	"ldflags":       {},
	"mod":           {},
	"modfile":       {},
	"overlay":       {},
	"p":             {},
	"pgo":           {},
	"pkgdir":        {},
	"tags":          {},
	"toolexec":      {},
}

// fixRun applies the suite's fixes until no fix is left, and returns an exit
// status. A round loads the packages, runs the suite over them, and writes the
// edits every analyzer suggested; a round that finds nothing is a fixed point.
func fixRun(args []string, analyzers []*analysis.Analyzer) int {
	err := run(args, analyzers)
	if err == nil {
		return exitOK
	}
	if msg := err.Error(); msg != "" {
		fmt.Fprintln(os.Stderr, "fix:", msg)
	}
	if errors.Is(err, errUsage) {
		return exitUsage
	}
	return exitFailed
}

// run is the command.
func run(args []string, analyzers []*analysis.Analyzer) error {
	opts, err := parseArgs(args, analyzers)
	if err != nil {
		return err
	}
	var (
		seen     = make(map[string]struct{}, maxRounds)
		unloaded int
	)
	for round := 1; round <= maxRounds; round++ {
		pending, bad, err := collect(opts)
		if err != nil {
			return err
		}
		if bad > 0 {
			unloaded = bad
		}
		if len(pending) == 0 {
			if unloaded > 0 {
				return unloadedError(unloaded)
			}
			if round == 1 {
				fmt.Fprintln(os.Stderr, "fix: nothing to fix")
			} else {
				fmt.Fprintf(os.Stderr, "fix: settled in %s\n", rounds(round-1))
			}
			return nil
		}
		var (
			names    = make([]string, 0, len(pending))
			seenName = make(map[string]struct{})
		)
		for _, f := range pending {
			if _, ok := seenName[f.name]; ok {
				continue
			}
			seenName[f.name] = struct{}{}
			names = append(names, f.name)
		}
		fmt.Fprintf(os.Stderr, "fix: round %d: %s\n", round,
			strings.Join(names, ", "),
		)
		// A round that proposes what an earlier round proposed cannot
		// make progress: two analyzers are undoing each other.
		key := pendingKey(pending)
		if _, ok := seen[key]; ok {
			return fmt.Errorf(
				"the same fixes are still pending after %s: %s in %s",
				rounds(round-1), strings.Join(names, ", "),
				strings.Join(pendingFiles(pending, 3), ", "),
			)
		}
		seen[key] = struct{}{}
		n, conflicts, err := write(pending)
		if err != nil {
			return err
		}
		for _, name := range conflicts {
			fmt.Fprintf(os.Stderr, "fix: conflicting fixes in %s\n", name)
		}
		if n == 0 {
			return fmt.Errorf("no fix could be applied: %s",
				strings.Join(names, ", "),
			)
		}
	}
	return fmt.Errorf("fixes did not settle in %d rounds", maxRounds)
}

// options is the command line: the analyzers to run and everything the go
// command needs to find the packages.
type options struct {
	analyzers []*analysis.Analyzer
	patterns  []string
	build     []string
	dir       string
}

// parseArgs splits the command line. An analyzer is selected by name, the way
// the go command selects one for vet: -NAME runs it, -NAME=false turns it off,
// and -NAME.flag=value sets one of its own flags. Everything else that starts
// with a dash is a build flag.
func parseArgs(args []string, analyzers []*analysis.Analyzer) (options, error) {
	p := newArgParser(analyzers)
	p.enable(args)
	for i := 0; i < len(args); i++ {
		next, err := p.arg(args, i)
		if err != nil {
			return p.opts, err
		}
		i = next
	}
	p.keep()
	// With no package named, the run covers the current directory, the way
	// go vet does.
	if len(p.opts.patterns) == 0 {
		p.opts.patterns = []string{"."}
	}
	return p.opts, nil
}

// help prints the analyzers and what each one reports.
func help(analyzers []*analysis.Analyzer) {
	for _, a := range analyzers {
		fmt.Printf("%s: %s\n", a.Name, a.Doc)
	}
}

// argParser is the state one command line is read into.
type argParser struct {
	opts      options
	byName    map[string]*analysis.Analyzer
	setByName map[string]*flag.FlagSet
	enabled   map[string]bool
}

// newArgParser returns a parser whose analyzers all start enabled.
func newArgParser(analyzers []*analysis.Analyzer) *argParser {
	p := &argParser{
		opts:      options{analyzers: analyzers},
		byName:    make(map[string]*analysis.Analyzer),
		setByName: make(map[string]*flag.FlagSet),
		enabled:   make(map[string]bool),
	}
	for _, a := range p.opts.analyzers {
		p.byName[a.Name] = a
		set := flag.NewFlagSet(a.Name, flag.ContinueOnError)
		a.Flags.VisitAll(func(f *flag.Flag) {
			set.Var(f.Value, f.Name, f.Usage)
		})
		p.setByName[a.Name] = set
	}
	return p
}

// enable decides which analyzers run. Naming one selects it, so the run
// covers exactly what was named; naming it false removes it from the whole
// suite.
func (p *argParser) enable(args []string) {
	var named bool
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name, value, hasValue := splitFlag(arg)
		if _, ok := p.byName[name]; ok && (!hasValue || value == "true") {
			named = true
		}
	}
	for name := range p.byName {
		p.enabled[name] = !named
	}
}

// keep drops the analyzers the command line turned off.
func (p *argParser) keep() {
	var kept []*analysis.Analyzer
	for _, a := range p.opts.analyzers {
		if p.enabled[a.Name] {
			kept = append(kept, a)
		}
	}
	p.opts.analyzers = kept
}

// arg reads one argument and returns the index of the last one it consumed.
// A flag that takes its value from the next argument consumes two.
func (p *argParser) arg(args []string, i int) (int, error) {
	arg := args[i]
	if !strings.HasPrefix(arg, "-") || arg == "-" {
		p.opts.patterns = append(p.opts.patterns, arg)
		return i, nil
	}
	name, value, hasValue := splitFlag(arg)
	if name == "C" {
		return p.dir(args, i, value, hasValue)
	}
	if _, ok := p.byName[name]; ok {
		return i, p.toggle(name, value, hasValue)
	}
	if base, sub, ok := strings.Cut(name, "."); ok {
		if _, ok := p.setByName[base]; ok {
			return p.setFlag(args, i, base, sub, value, hasValue)
		}
	}
	// A flag in unsupported means nothing to the load, so forwarding it
	// would print a usage message about the wrong tool.
	if _, ok := unsupported[name]; ok {
		if name == "h" || name == "help" {
			help(p.opts.analyzers)
			os.Exit(exitOK)
		}
		return i, fmt.Errorf("%w: -%s is not supported", errUsage, name)
	}
	// A build flag that takes a separate value needs both halves. Left
	// alone, the value reads as a package pattern and the load fails on a
	// path that was never a path.
	if _, takes := buildValueFlags[name]; takes && !hasValue {
		return p.twoPart(args, i, arg)
	}
	p.opts.build = append(p.opts.build, arg)
	return i, nil
}

// splitFlag returns a flag's name and value, with or without an equals sign.
func splitFlag(arg string) (name, value string, hasValue bool) {
	return strings.Cut(strings.TrimLeft(arg, "-"), "=")
}

// dir reads -C, which names the directory the load runs in.
func (p *argParser) dir(args []string, i int, value string, hasValue bool) (int, error) {
	if !hasValue {
		if i+1 >= len(args) {
			return i, fmt.Errorf("%w: -C needs a directory", errUsage)
		}
		value = args[i+1]
		i++
	}
	p.opts.dir = value
	return i, nil
}

// toggle reads an analyzer's name, which selects it or removes it.
func (p *argParser) toggle(name, value string, hasValue bool) error {
	on := true
	if hasValue {
		switch value {
		case "true":
		case "false":
			on = false
		default:
			return fmt.Errorf("%w: -%s takes true or false", errUsage, name)
		}
	}
	p.enabled[name] = on
	return nil
}

// setFlag reads one of an analyzer's own flags, written name.sub.
func (p *argParser) setFlag(args []string, i int, base, sub, value string, hasValue bool) (int, error) {
	if !hasValue {
		if i+1 >= len(args) {
			return i, fmt.Errorf("%w: -%s needs a value", errUsage,
				base+"."+sub,
			)
		}
		value = args[i+1]
		i++
	}
	if err := p.setByName[base].Set(sub, value); err != nil {
		return i, fmt.Errorf("%w: %w", errUsage, err)
	}
	return i, nil
}

// twoPart reads a build flag whose value is the next argument.
func (p *argParser) twoPart(args []string, i int, arg string) (int, error) {
	if i+1 >= len(args) {
		return i, fmt.Errorf("%w: %s needs a value", errUsage, arg)
	}
	p.opts.build = append(p.opts.build, arg, args[i+1])
	return i + 1, nil
}

// span is a range of a file that an accepted edit covers.
type span struct{ lo, hi int }

// edit is one replacement, in byte offsets so it can be spliced into a file
// without a position table. Edits that come from one suggested fix share a
// group, and a group is applied whole or not at all.
type edit struct {
	lo, hi int
	group  int
	text   []byte
}

// fixes is one analyzer's edits, keyed by file.
type fixes struct {
	name  string
	files map[string][]edit
}

// collect runs the suite and returns the edits it suggests, one analyzer's
// worth at a time and in suite order.
func collect(opts options) ([]fixes, int, error) {
	if len(opts.analyzers) == 0 {
		return nil, 0, nil
	}
	loaded, bad, err := load(opts)
	if err != nil {
		return nil, 0, err
	}
	graph, err := checker.Analyze(ignore.Apply(opts.analyzers...), loaded, nil)
	if err != nil {
		return nil, bad, err
	}
	var out []fixes
	for _, act := range graph.Roots {
		if act.Package == nil || len(act.Diagnostics) == 0 {
			continue
		}
		var (
			f = fixes{
				name:  act.Analyzer.Name,
				files: make(map[string][]edit),
			}
			group int
		)
		for _, d := range act.Diagnostics {
			for _, s := range d.SuggestedFixes {
				group++
				for _, te := range s.TextEdits {
					tf := act.Package.Fset.File(te.Pos)
					if tf == nil {
						continue
					}
					f.files[tf.Name()] = append(f.files[tf.Name()], edit{
						lo:    tf.Offset(te.Pos),
						hi:    tf.Offset(te.End),
						group: group,
						text:  te.NewText,
					})
				}
			}
		}
		if len(f.files) > 0 {
			out = append(out, f)
		}
	}
	return out, bad, nil
}

// load reads the packages the analyzers will see.
func load(opts options) ([]*packages.Package, int, error) {
	mode := packages.LoadSyntax
	if needsFacts(opts.analyzers) {
		mode = packages.LoadAllSyntax
	}
	cfg := &packages.Config{
		Mode:       mode | packages.NeedModule,
		Tests:      true,
		BuildFlags: opts.build,
		Dir:        opts.dir,
	}
	loaded, err := packages.Load(cfg, opts.patterns...)
	if err != nil {
		return nil, 0, err
	}
	if len(loaded) == 0 {
		return nil, 0, fmt.Errorf("%s matched no packages",
			strings.Join(opts.patterns, " "),
		)
	}
	// A package that will not load is left alone: cgo and assembly packages
	// need a compiler that the run may not have. The go command prints
	// why, and the run ends with a non-zero status, so a tree that does not
	// build is never reported as fixed.
	loaded = roots(loaded)
	packages.PrintErrors(loaded)
	var bad int
	for _, p := range loaded {
		if len(p.Errors) > 0 {
			bad++
		}
	}
	return loaded, bad, nil
}

// write applies the round's edits a fix at a time, in suite order. A fix
// whose edits overlap what is already applied is left for the next round, so
// an analyzer that rewrites a span wins over one that rewrites inside it, and
// no fix is ever applied in part.
func write(pending []fixes) (int, []string, error) {
	var (
		accepted = make(map[string][]edit)
		taken    = make(map[string][]span)
		applied  int
	)
	for _, f := range pending {
		for name, edits := range f.files {
			for _, group := range groups(edits) {
				if conflicts(taken[name], group) {
					continue
				}
				accepted[name] = append(accepted[name], group...)
				for _, e := range group {
					taken[name] = append(taken[name], span{e.lo, e.hi})
				}
				applied++
			}
		}
	}
	names := make([]string, 0, len(accepted))
	for name := range accepted {
		names = append(names, name)
	}
	sort.Strings(names)
	// Two analyzers can suggest the same edit, and an import added by
	// both would land twice.
	for name, edits := range accepted {
		accepted[name] = dedupe(edits)
	}
	var clashed []string
	for _, name := range names {
		src, err := os.ReadFile(name)
		if err != nil {
			return applied, clashed, err
		}
		out, ok := apply(src, accepted[name])
		if !ok {
			clashed = append(clashed, name)
			continue
		}
		// A fix that drops a use leaves the import behind, and the next
		// round cannot type-check a package with an unused import.
		if fixed, err := imports.Process(name, out, nil); err == nil {
			out = fixed
		} else {
			fmt.Fprintf(os.Stderr, "fix: %s: %v\n", name, err)
		}
		if err := os.WriteFile(name, out, 0o666); err != nil {
			return applied, clashed, err
		}
	}
	return applied, clashed, nil
}

// dedupe drops edits that repeat one already taken.
func dedupe(edits []edit) []edit {
	var (
		out  = edits[:0]
		seen = make(map[string]struct{}, len(edits))
	)
	for _, e := range edits {
		key := fmt.Sprintf("%d\x00%d\x00%s", e.lo, e.hi, e.text)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e)
	}
	return out
}

// pendingFiles returns up to n files a round would rewrite.
func pendingFiles(pending []fixes, n int) (out []string) {
	for _, f := range pending {
		for name := range f.files {
			out = append(out, name)
			if len(out) >= n {
				return out
			}
		}
	}
	return out
}

// pendingKey identifies a round's proposed fixes, so a repeat is visible.
func pendingKey(pending []fixes) string {
	var b strings.Builder
	for _, f := range pending {
		b.WriteString(f.name)
		var names []string
		for name := range f.files {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			b.WriteString(fixKey(name, f.files[name]))
		}
	}
	return b.String()
}

// groups returns the edits of one file split by suggested fix, in the order
// they arrived.
func groups(edits []edit) (out [][]edit) {
	last := -1
	for _, e := range edits {
		if e.group != last {
			out = append(out, nil)
			last = e.group
		}
		out[len(out)-1] = append(out[len(out)-1], e)
	}
	return out
}

// fixKey is the identity of one fix: its file and every edit in it.
func fixKey(name string, group []edit) string {
	var b strings.Builder
	b.WriteString(name)
	for _, e := range group {
		fmt.Fprintf(&b, "\x00%d\x00%d\x00%s", e.lo, e.hi, e.text)
	}
	return b.String()
}

// conflicts reports whether any edit in group overlaps a taken span.
func conflicts(taken []span, group []edit) bool {
	for _, e := range group {
		if overlaps(taken, e) {
			return true
		}
	}
	return false
}

// overlaps reports whether e touches a span that is already spoken for. Two
// insertions at one offset do not touch, and neither does an insertion at the
// edge of a replacement.
func overlaps(taken []span, e edit) bool {
	for _, s := range taken {
		if e.lo < s.hi && s.lo < e.hi {
			return true
		}
	}
	return false
}

// apply splices edits into src. It reports false when two edits overlap.
func apply(src []byte, edits []edit) ([]byte, bool) {
	// Highest offset first, and at one offset the widest edit first: an
	// insertion at the start of a replacement is still correct after the
	// replacement, but the reverse is not.
	slices.SortFunc(edits, func(a, b edit) int {
		if a.lo != b.lo {
			return b.lo - a.lo
		}
		return b.hi - a.hi
	})
	out := src
	for i, e := range edits {
		if e.lo < 0 || e.hi > len(out) || e.lo > e.hi {
			return nil, false
		}
		if i > 0 && e.lo < edits[i-1].hi && edits[i-1].lo < e.hi {
			return nil, false
		}
		next := make([]byte, 0, len(out)-(e.hi-e.lo)+len(e.text))
		next = append(next, out[:e.lo]...)
		next = append(next, e.text...)
		next = append(next, out[e.hi:]...)
		out = next
	}
	return out, true
}

func rounds(n int) string {
	if n == 1 {
		return "1 round"
	}
	return fmt.Sprintf("%d rounds", n)
}

// unloadedError reports packages that did not load. The fixes that could be
// applied were applied, but the loop did not reach a fixed point: a package in
// the tree does not build.
func unloadedError(n int) error {
	if n == 1 {
		return fmt.Errorf("1 package did not load")
	}
	return fmt.Errorf("%d packages did not load", n)
}
