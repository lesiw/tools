package unwrap

import (
	"strings"
	"testing"
)

// The rules have golden fixtures of their own; this checks the wiring: a
// wrapped construct that fits is joined, and one that does not is left.
func TestJoin(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{{
		name: "a call that fits",
		src:  "package p\n\nfunc f() {\n\tg(a,\n\t\tb)\n}\n",
		want: "package p\n\nfunc f() {\n\tg(a, b)\n}\n",
	}, {
		name: "a call that does not fit",
		src: "package p\n\nfunc f() {\n\tg(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa,\n" +
			"\t\tbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb)\n}\n",
		want: "package p\n\nfunc f() {\n\tg(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa,\n" +
			"\t\tbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb)\n}\n",
	}}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			out, err := applyAll("x.go", []byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != tt.want {
				t.Errorf("Join() =\n%s\nwant:\n%s", out, tt.want)
			}
		})
	}
}

// TestAnalyzer checks that the analyzer is the one the suite names.
func TestAnalyzer(t *testing.T) {
	if Analyzer.Name != "unwrap" {
		t.Errorf("Name = %q", Analyzer.Name)
	}
	if !strings.Contains(Analyzer.Doc, "wrapped") {
		t.Errorf("Doc = %q", Analyzer.Doc)
	}
}
