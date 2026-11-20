package templateparse

import (
	"embed"
	"errors"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	analysistest.Run(t, fixtures.Dir(t, testdata), Analyzer, "a")
}

// TestDetail pins the shape of the error that New("") gives, which the message
// is read from.
func TestDetail(t *testing.T) {
	cases := []struct {
		text string
		line int
		msg  string
	}{
		{`{{end}}`, 1, "unexpected {{end}}"},
		{"alpha\n{{end}}", 2, "unexpected {{end}}"},
		{`{{.Kind} {{.Label}}`, 1, "bad character U+007D '}'"},
	}
	for _, tc := range cases {
		err := parse("text/template", tc.text, templateDefaults)
		if err == nil {
			t.Fatalf("parse(%q) = nil, want an error", tc.text)
		}
		line, msg, ok := detail(err)
		if !ok {
			t.Errorf("detail(%v) is not known", err)
			continue
		}
		if line != tc.line || msg != tc.msg {
			t.Errorf(
				"detail(%v) = %d, %q, want %d, %q",
				err, line, msg, tc.line, tc.msg,
			)
		}
	}
	if _, _, ok := detail(errors.New("alpha")); ok {
		t.Error("detail(an unrelated error) is known")
	}
}
