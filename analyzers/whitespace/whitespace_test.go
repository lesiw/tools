package whitespace

import "testing"

// A blank line appears between a multi-line declaration and its neighbor.
func TestSpace(t *testing.T) {
	var (
		src  = "package p\n\nfunc a() {}\n\nfunc b() {\n\tprintln(1)\n}\nfunc c() {}\n"
		want = "package p\n\nfunc a() {}\n\nfunc b() {\n\tprintln(1)\n}\n\nfunc c() {}\n"
	)
	out, err := space("x.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != want {
		t.Errorf("Space() =\n%s\nwant:\n%s", out, want)
	}
}

// TestAnalyzer checks that the analyzer is the one the suite names.
func TestAnalyzer(t *testing.T) {
	if Analyzer.Name != "whitespace" {
		t.Errorf("Name = %q", Analyzer.Name)
	}
}
