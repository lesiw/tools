package main

import (
	"os"
	"testing"
)

func TestRun(t *testing.T) {
	for _, fileset := range []string{"app", "lib"} {
		t.Run(fileset, func(t *testing.T) {
			t.Chdir(t.TempDir())
			for range 3 {
				if err := run([]string{fileset}); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{".editorconfig", "clerk.sum"} {
					if _, err := os.Stat(name); err != nil {
						t.Errorf("missing %s: %v", name, err)
					}
				}
			}
		})
	}
}

func TestRunBadArgs(t *testing.T) {
	if err := run(nil); err == nil {
		t.Error("run(nil): want error, got nil")
	}
	err := run([]string{"foo"})
	if err == nil {
		t.Error(`run("foo"): want error, got nil`)
	}
}
