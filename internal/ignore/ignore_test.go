package ignore

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

var analysistestCases = []struct {
	name      string
	dir       string
	analyzers []*analysis.Analyzer
}{
	{"Empty", "empty", nil},
	{"Single", "single", []*analysis.Analyzer{publicNames}},
	{"Basic", "basic", names},
	{"Ignore", "ignore", names},
	{"MultipleIgnore", "multiple", names},
	{"InlineIgnore", "inline", names},
	{"BlockIgnore", "block", names},
	{"FileLevelIgnore", "filelevel", names},
	{"BlockLevelIgnore", "blocklevel", short},
	{"MixedBlockIgnore", "mixedblock", all},
	{"CommentIgnore", "commentignore", todos},
	{"FileStartIgnore", "filestart", todos},
	{"LineStartIgnore", "linestart", todos},
	{"LineEndIgnore", "lineend", []*analysis.Analyzer{dotAnalyzer}},
	{"Dependencies", "dependency", []*analysis.Analyzer{dependentAnalyzer}},
	{"GeneratedFiles", "generated", []*analysis.Analyzer{publicNames}},
}

func TestAnalyzers(t *testing.T) {
	for _, tt := range analysistestCases {
		t.Run(tt.name, func(t *testing.T) {
			analysistest.Run(
				t, analysistest.TestData(),
				combined(tt.analyzers...), tt.dir,
			)
		})
	}
}

func TestParseIgnore(t *testing.T) {
	var (
		allOnly   = map[string]struct{}{"all": {}}
		test1Only = map[string]struct{}{"test1": {}}
		test1And2 = map[string]struct{}{"test1": {}, "test2": {}}
		tests     = []struct {
			input string
			want  map[string]struct{}
			in    bool
		}{
			{"//ignore", allOnly, false},
			{"//ignore:all", allOnly, false},
			{"//ignore:test1", test1Only, false},
			{"//ignore:test1,test2", test1And2, false},
			{"//ignore:all // This is a comment", allOnly, false},
			{"//ignore:test1 // Explanatory comment", test1Only, false},
			{"//ignore:test1,test2 // Multiple analyzers", test1And2, false},
			{"// not ignore", nil, false},
			{"//other", nil, false},
			{"// This is a comment. //ignore:all", allOnly, true},
			{"// a comment //ignore:all // and another", allOnly, true},
		}
	)
	for _, tt := range tests {
		gotAnalyzers, gotIn := parseIgnore(tt.input)
		switch {
		case tt.want == nil && gotAnalyzers != nil:
			t.Errorf("parseIgnore(%q) = %v, want nil", tt.input, gotAnalyzers)
		case gotAnalyzers == nil && tt.want != nil:
			t.Errorf("parseIgnore(%q) = nil, want %v", tt.input, tt.want)
		case !cmp.Equal(gotAnalyzers, tt.want):
			t.Errorf(
				"parseIgnore(%q) mismatch (-want +got):\n%s",
				tt.input, cmp.Diff(tt.want, gotAnalyzers),
			)
		case gotIn != tt.in:
			t.Errorf(
				"parseIgnore(%q) in = %t, want %t",
				tt.input, gotIn, tt.in,
			)
		}
	}
}

type factA struct{}

func (*factA) AFact() {}

func TestFilterKeepsAnalyzers(t *testing.T) {
	var (
		flag bool
		noop = func(*analysis.Pass) (any, error) { return nil, nil }
		dep  = &analysis.Analyzer{Name: "dep", Doc: "dep", Run: noop}
		a    = &analysis.Analyzer{
			Name:      "a",
			Doc:       "a",
			Requires:  []*analysis.Analyzer{dep},
			FactTypes: []analysis.Fact{new(factA)},
			Run:       noop,
		}
	)
	a.Flags.BoolVar(&flag, "flag", false, "a flag")
	got := Apply(a)
	if len(got) != 1 {
		t.Fatalf("Apply(a) = %d analyzers, want 1", len(got))
	}
	if got[0].Name != a.Name || got[0].Doc != a.Doc {
		t.Errorf(
			"Apply(a)[0] = %q, %q, want %q, %q",
			got[0].Name, got[0].Doc, a.Name, a.Doc,
		)
	}
	if got[0].Flags.Lookup("flag") == nil {
		t.Error(`Apply(a)[0].Flags.Lookup("flag") = nil, want flag`)
	}
	if !slices.Equal(got[0].FactTypes, a.FactTypes) {
		t.Errorf(
			"Apply(a)[0].FactTypes = %v, want %v",
			got[0].FactTypes, a.FactTypes,
		)
	}
	reqs := got[0].Requires
	if want := []*analysis.Analyzer{dep, ignore}; !slices.Equal(reqs, want) {
		t.Errorf("Apply(a)[0].Requires = %v, want %v", reqs, want)
	}
	if len(a.Requires) != 1 {
		t.Errorf("len(a.Requires) = %d, want 1", len(a.Requires))
	}
}
