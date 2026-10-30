package reflow

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// reflow returns what the command writes to standard output for src.
func reflow(t *testing.T, src string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "in.go")
	if err := os.WriteFile(name, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = f
	t.Cleanup(func() {
		os.Stdin = old
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldOut })
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		if _, err := io.Copy(&b, r); err != nil {
			t.Error(err)
		}
		done <- b.String()
	}()
	if err := run(nil); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldOut
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return <-done
}

func TestRun(t *testing.T) {
	const (
		src = `package p

// Do performs the operation and returns a value, and this comment is long enough that the reflow rules wrap it onto two lines.
func Do() int {
	x := 1 // a trailing comment long enough that it cannot stay on the line it trails
	return x
}
`
		want = `package p

// Do performs the operation and returns a value, and this comment is long
// enough that the reflow rules wrap it onto two lines.
func Do() int {
	// a trailing comment long enough that it cannot stay on the line it trails
	x := 1
	return x
}
`
	)
	if got := reflow(t, src); got != want {
		t.Errorf("reflow = %q, want %q", got, want)
	}
}

func TestFilesRefused(t *testing.T) {
	if err := run([]string{"x.go"}); err == nil {
		t.Error("reflow accepted a file argument")
	}
}
