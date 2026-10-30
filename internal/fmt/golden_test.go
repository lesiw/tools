package fmt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGolden holds each fixture to the file one pass of the rules leaves
// behind. A fixture whose input already holds a long line keeps it: the
// formatter may leave a long line, and may not make one.
func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.input")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no fixtures")
	}
	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Format(src)
			if err != nil {
				t.Fatal(err)
			}
			golden := strings.TrimSuffix(input, ".input") + ".golden"
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("fmt left:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
