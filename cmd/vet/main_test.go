package main

import (
	"errors"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"lesiw.io/command"
	"lesiw.io/command/mock"
)

func swap[T any](t *testing.T, orig *T, with T) {
	t.Helper()
	o := *orig
	t.Cleanup(func() { *orig = o })
	*orig = with
}

func TestRunExecsGoVet(t *testing.T) {
	mm := new(mock.Machine)
	swap(t, &m, command.Machine(mm))
	swap(t, &executable, func() (string, error) { return "/tool/vet", nil })
	swap(t, &os.Args, []string{"vet", "-printf=false", "./..."})
	if err := run(t.Context()); err != nil {
		t.Fatalf("run() = %v; want nil", err)
	}
	want := []mock.Call{{
		Args: []string{
			"go", "vet", "-vettool=/tool/vet", "-printf=false", "./...",
		},
		Env: map[string]string{"GOVET": "1"},
	}}
	if !cmp.Equal(want, mm.Calls) {
		t.Errorf("calls: -want +got\n%s", cmp.Diff(want, mm.Calls))
	}
}

func TestRunReportsFailure(t *testing.T) {
	mm := new(mock.Machine)
	swap(t, &m, command.Machine(mm))
	swap(t, &executable, func() (string, error) { return "/tool/vet", nil })
	swap(t, &os.Args, []string{"vet", "./..."})
	mm.Return(command.Fail(&command.Error{Code: 1}),
		"go", "vet", "-vettool=/tool/vet", "./...",
	)
	err := run(t.Context())
	cmdErr, ok := errors.AsType[*command.Error](err)
	if !ok || cmdErr.Code != 1 {
		t.Fatalf("run() = %v; want exit code 1", err)
	}
}
