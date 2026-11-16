package regexpraw

import (
	"embed"
	"go/ast"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// TestRawLiteral checks which interpreted literals have a raw form.
func TestRawLiteral(t *testing.T) {
	tests := []struct {
		name, value, raw string
		ok               bool
	}{{
		name:  "doubled backslashes",
		value: `"\\d+"`,
		raw:   "`\\d+`",
		ok:    true,
	}, {
		name:  "two doubled backslashes",
		value: `"\\d\\t"`,
		raw:   "`\\d\\t`",
		ok:    true,
	}, {
		name:  "single escape",
		value: `"\n"`,
	}, {
		name:  "another escape",
		value: `"\\d\n"`,
	}, {
		name:  "backtick",
		value: "\"\\\\d`\"",
	}, {
		name:  "carriage return",
		value: "\"\\\\d\r\"",
	}, {
		name:  "raw already",
		value: "`\\d+`",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				lit     = &ast.BasicLit{Kind: token.STRING, Value: tt.value}
				raw, ok = rawLiteral(lit)
			)
			if raw != tt.raw || ok != tt.ok {
				t.Errorf(
					"rawLiteral(%q) = %q, %v; want %q, %v",
					tt.value, raw, ok, tt.raw, tt.ok,
				)
			}
		})
	}
}
