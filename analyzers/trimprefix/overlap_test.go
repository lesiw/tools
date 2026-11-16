package trimprefix

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/modernize"

	"lesiw.io/tools/internal/fixtures"
)

// TestOverlap checks that stringscutprefix reports each shape this rule leaves
// to it.
func TestOverlap(t *testing.T) {
	var (
		dir  = t.TempDir()
		file = filepath.Join(dir, "a.go")
	)
	fixtures.Module(t, dir, "a", "1.20")
	src := `package a

import (
	"bytes"
	"strings"
)

type item struct{ name string }

func identifier(s, prefix string) string {
	if strings.HasPrefix(s, prefix) { // want "CutPrefix"
		s = strings.TrimPrefix(s, prefix)
	}
	return s
}

func bytesPair(b, prefix []byte) []byte {
	if bytes.HasPrefix(b, prefix) { // want "CutPrefix"
		b = bytes.TrimPrefix(b, prefix)
	}
	return b
}

func selector(it *item, prefix string) {
	if strings.HasPrefix(it.name, prefix) { // want "CutPrefix"
		it.name = strings.TrimPrefix(it.name, prefix)
	}
}

func indexed(xs []string, prefix string) []string {
	if strings.HasPrefix(xs[0], prefix) { // want "CutPrefix"
		xs[0] = strings.TrimPrefix(xs[0], prefix)
	}
	return xs
}

func pointed(s *string, prefix string) {
	if strings.HasPrefix(*s, prefix) { // want "CutPrefix"
		*s = strings.TrimPrefix(*s, prefix)
	}
}

func literal(s string) string {
	if strings.HasPrefix(s, "some-prefix") { // want "CutPrefix"
		s = strings.TrimPrefix(s, "some-prefix")
	}
	return s
}

func chained(s, prefix string, done bool) string {
	if done {
		return s
	} else if strings.HasPrefix(s, prefix) { // want "CutPrefix"
		s = strings.TrimPrefix(s, prefix)
	}
	return s
}

func mapped(m map[string]string, key, prefix string) {
	if strings.HasPrefix(m[key], prefix) { // want "CutPrefix"
		m[key] = strings.TrimPrefix(m[key], prefix)
	}
}

func arrayed(xs [1]string, prefix string) [1]string {
	if strings.HasPrefix(xs[0], prefix) { // want "CutPrefix"
		xs[0] = strings.TrimPrefix(xs[0], prefix)
	}
	return xs
}

func pointedArray(xs *[1]string, prefix string) {
	if strings.HasPrefix(xs[0], prefix) { // want "CutPrefix"
		xs[0] = strings.TrimPrefix(xs[0], prefix)
	}
}

func sliceValue(m map[string][]string, key, prefix string) {
	if strings.HasPrefix(m[key][0], prefix) { // want "CutPrefix"
		m[key][0] = strings.TrimPrefix(m[key][0], prefix)
	}
}

func pointedValue(m map[string]*[1]string, key, prefix string) {
	if strings.HasPrefix(m[key][0], prefix) { // want "CutPrefix"
		m[key][0] = strings.TrimPrefix(m[key][0], prefix)
	}
}

func generic[T ~[]string](xs T, prefix string) T {
	if strings.HasPrefix(xs[0], prefix) { // want "CutPrefix"
		xs[0] = strings.TrimPrefix(xs[0], prefix)
	}
	return xs
}

func genericUnion[T ~[]string | ~[1]string](xs T, prefix string) T {
	if strings.HasPrefix(xs[0], prefix) { // want "CutPrefix"
		xs[0] = strings.TrimPrefix(xs[0], prefix)
	}
	return xs
}

func parenthesized(s, prefix string) string {
	if (strings.HasPrefix)(s, prefix) { // want "CutPrefix"
		s = (strings.TrimPrefix)(s, prefix)
	}
	return s
}

func spread(s, prefix string) string {
	if strings.HasPrefix( // want "CutPrefix"
		s,
		prefix,
	) {
		s = strings.TrimPrefix(s, prefix)
	}
	return s
}

func labeled(s, prefix string) string {
again:
	if strings.HasPrefix(s, prefix) { // want "CutPrefix"
		s = strings.TrimPrefix(s, prefix)
	}
	if s == "some-value" {
		goto again
	}
	return s
}
`
	if err := os.WriteFile(file, []byte(src), 0o666); err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, dir, modernize.StringsCutPrefixAnalyzer, "a")
}
