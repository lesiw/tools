package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGolden holds each fixture to the text a reflow leaves behind.
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
			got, err := Reflow(src)
			if err != nil {
				t.Fatal(err)
			}
			golden := strings.TrimSuffix(input, ".input") + ".golden"
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("reflow left:\n%s\nwant:\n%s", got, want)
			}
			// A second run must change nothing: the command is a formatter,
			// and its output is what a reader is left with.
			again, err := Reflow(got)
			if err != nil {
				t.Fatal(err)
			}
			if string(again) != string(got) {
				t.Errorf("a second run changed the output:\n%s", again)
			}
		})
	}
}

// TestGrowingGroupHoldsIndent holds a group that grows to the indent of the
// group it follows. A later comment measured against the lines the run
// started with would lose its indent.
func TestGrowingGroupHoldsIndent(t *testing.T) {
	const (
		src = `package p

// A is a doc comment that is long enough that the reflow rules should wrap it onto two lines, definitely.
func A() {}

var (
	// B is a spec doc that is long enough that the reflow rules should wrap it onto two lines.
	B = 1
)
`
		want = `package p

// A is a doc comment that is long enough that the reflow rules should wrap it
// onto two lines, definitely.
func A() {}

var (
	// B is a spec doc that is long enough that the reflow rules should wrap it
	// onto two lines.
	B = 1
)
`
	)
	got, err := Reflow([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("reflow gave:\n%s\nwant:\n%s", got, want)
	}
}
