package fix

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"lesiw.io/tools/internal/tool"
)

type argCase struct {
	name string
	args []string
	vet  string
	want []string
}

var argCases = []argCase{{
	name: "packages only",
	args: []string{"./..."},
	vet:  tool.Suite,
	want: []string{"./..."},
}, {
	name: "no package named means this directory",
	args: nil,
	vet:  tool.Suite,
	want: []string{"."},
}, {
	name: "analysis tool as two words",
	args: []string{"-vettool", "x/y", "./..."},
	vet:  "x/y",
	want: []string{"./..."},
}, {
	name: "analysis tool as one word",
	args: []string{"-vettool=x/y", "./..."},
	vet:  "x/y",
	want: []string{"./..."},
}, {
	name: "analysis tool with two hyphens",
	args: []string{"--vettool=x/y", "./..."},
	vet:  "x/y",
	want: []string{"./..."},
}, {
	name: "an analyzer flag is passed on",
	args: []string{"-stringsseq=false", "-tags=netgo", "./..."},
	vet:  tool.Suite,
	want: []string{"-stringsseq=false", "-tags=netgo", "./..."},
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
			if !slices.Equal(opts.args, tt.want) {
				t.Errorf("args = %q, want %q", opts.args, tt.want)
			}
		})
	}
}

func TestParseArgsError(t *testing.T) {
	if _, err := parseArgs([]string{"-vettool"}); err == nil {
		t.Error("parseArgs accepted -vettool with no value")
	}
}

func TestGoCommand(t *testing.T) {
	opts, err := parseArgs([]string{"-C", "/tmp", "-tags=netgo", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	var (
		argv = opts.goCommand("vet", "-json").Args[1:]
		want = []string{"-C", "/tmp", "vet", "-json", "-tags=netgo", "./..."}
	)
	if !slices.Equal(argv, want) {
		t.Errorf("goCommand = %q, want %q", argv, want)
	}
}

func TestParseReport(t *testing.T) {
	const report = `{"m": {"stringsseq": [
		{"posn": "a.go:6:20", "message": "ranging",
		 "suggested_fixes": [{"message": "use FieldsSeq"}]},
		{"posn": "a.go:9:2", "message": "no fix here"}
	], "strictvar": [
		{"posn": "a.go:22:1", "message": "combine",
		 "suggested_fixes": [{"message": "combine them"}]}
	]}}`
	got, err := parseReport(strings.NewReader(report))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"strictvar\ta.go:22:1: combine them",
		"stringsseq\ta.go:6:20: use FieldsSeq",
	}
	if !slices.Equal(got, want) {
		t.Errorf("parseReport = %q, want %q", got, want)
	}
}

func TestParseReportEmpty(t *testing.T) {
	got, err := parseReport(strings.NewReader(""))
	if err != nil || len(got) != 0 {
		t.Errorf("parseReport(empty) = %q, %v", got, err)
	}
}

func TestRounds(t *testing.T) {
	for n, want := range map[int]string{1: "1 round", 2: "2 rounds"} {
		if got := rounds(n); got != want {
			t.Errorf("rounds(%d) = %q, want %q", n, got, want)
		}
	}
}

// fakeDriver reports a list of analyzers per round and applies what it is
// asked to.
type fakeDriver struct {
	reports [][]string
	applied []string
	fail    error
}

func (f *fakeDriver) report(string) ([]string, error) {
	if len(f.reports) == 0 {
		return nil, nil
	}
	next := f.reports[0]
	f.reports = f.reports[1:]
	return next, nil
}

func (f *fakeDriver) apply(_, name string) error {
	if f.fail != nil {
		return f.fail
	}
	f.applied = append(f.applied, name)
	return nil
}

func TestRunSettles(t *testing.T) {
	f := &fakeDriver{reports: [][]string{
		{"stringsseq\ta.go:6:20: use FieldsSeq"},
		nil,
	}}
	if err := loop(f, "vet"); err != nil {
		t.Fatalf("run = %v, want nil", err)
	}
	if want := []string{"stringsseq"}; !slices.Equal(f.applied, want) {
		t.Errorf("applied = %q, want %q", f.applied, want)
	}
}

func TestRunNothingToFix(t *testing.T) {
	f := new(fakeDriver)
	if err := loop(f, "vet"); err != nil {
		t.Fatalf("run = %v, want nil", err)
	}
	if len(f.applied) != 0 {
		t.Errorf("applied = %q, want nothing", f.applied)
	}
}

func TestRunAppliesEachAnalyzerOnce(t *testing.T) {
	f := &fakeDriver{reports: [][]string{{
		"slicescontains\ta.go:6:2: use slices.Contains",
		"stringsseq\ta.go:6:20: use FieldsSeq",
		"strictvar\ta.go:22:1: combine",
	}, nil}}
	if err := loop(f, "vet"); err != nil {
		t.Fatalf("run = %v, want nil", err)
	}
	want := []string{"slicescontains", "stringsseq", "strictvar"}
	if !slices.Equal(f.applied, want) {
		t.Errorf("applied = %q, want %q", f.applied, want)
	}
}

func TestRunStopsOnARepeatedReport(t *testing.T) {
	var (
		report = []string{"strictvar\ta.go:22:1: combine"}
		f      = &fakeDriver{reports: [][]string{report, report}}
	)
	err := loop(f, "vet")
	if err == nil || !strings.Contains(err.Error(), "still pending") {
		t.Fatalf("run = %v, want a stall", err)
	}
}

func TestRunReportsAnApplyFailure(t *testing.T) {
	f := &fakeDriver{
		reports: [][]string{{"strictvar\ta.go:22:1: combine"}},
		fail:    fmt.Errorf("conflict"),
	}
	if err := loop(f, "vet"); err == nil {
		t.Fatal("run = nil, want the apply failure")
	}
}

// skipDriver reports one analyzer per round and fails each apply the way the
// go command fails a skipped fix.
type skipDriver struct {
	reports [][]string
	applied []string
}

func (s *skipDriver) report(string) ([]string, error) {
	if len(s.reports) == 0 {
		return nil, nil
	}
	next := s.reports[0]
	s.reports = s.reports[1:]
	return next, nil
}

func (s *skipDriver) apply(_, name string) error {
	s.applied = append(s.applied, name)
	return fmt.Errorf("fix with %s: exit status 1: applied 3 of 4 fixes; 1 file updated. (Re-run the command to apply more.)", name)
}

func TestRunSetsAsideAnAnalyzerThatSkips(t *testing.T) {
	f := &skipDriver{reports: [][]string{
		{"strictvar\ta.go:1:1: combine"},
		{"strictvar\ta.go:1:1: combine"},
	}}
	err := loop(f, "vet")
	if err == nil || !strings.Contains(err.Error(), "still pending") {
		t.Fatalf("run = %v, want the pending fix named", err)
	}
	if len(f.applied) != 1 {
		t.Errorf("applied = %q, want one attempt", f.applied)
	}
}
