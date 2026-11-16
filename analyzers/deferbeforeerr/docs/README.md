# lesiw.io/tools/analyzers/deferbeforeerr

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deferbeforeerr.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deferbeforeerr)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a deferred call made before the error
result is read.

```go
func f() error {
	h, err := open("some-name")
	defer h.Close() // h.Close() is deferred before err is checked
	if err != nil {
		return err
	}
	return nil
}
```

The spec's Defer statements section says the function value and the arguments
to the call are evaluated when the defer executes. The value is taken there, so
the deferred `Close` is registered on whatever `open` returned, and the `if`
that returns the error runs afterward. When `open` fails, the deferred call
runs on the value it returned anyway.

The net/http documentation for `Client.Do` says when one such value may be
ignored: "On error, any Response can be ignored."

## Checks

The rule reads an assignment of one call whose result count matches its
left-hand side and whose last result is the predeclared `error` type. A `var`
declaration that binds the same results is not read. Every deferred call below
the assignment in the same statement list that names one of the other results
is reported, as a receiver or as an argument.

The deferred call does not have to follow the assignment immediately. The rule
reads past the statements in between and stops at the first one that reads the
error, since the error has been answered by then. An assignment to one of the
values stops it too: the error no longer decides the value bound there.

A discarded error is never read, so the rule runs to the end of the statement
list and reports every deferred call that names one of the values.

The rule reads one statement list at a time: a block, a switch case, or a
select case. A deferred call inside the body of an `if` or a loop below the
assignment is left alone.

A deferred function literal is left alone. The rule reads the receiver and the
arguments of the call the defer registers, and a literal names the value in its
body instead, where the read happens at return. The value there can be nil in
the same way; `deferunlambda` rewrites the literal whose body is a single call.

A trailing result that is not the predeclared `error` is left alone, so a call
that reports a value and a `bool` is not read. The error result written to
something other than an identifier is left alone too, since the rule cannot
follow it from there.

The rule is adapted from staticcheck's SA5001, which reads a deferred `Close`
on the identifier the assignment bound and leaves a discarded error alone.

Report only: the fix moves the deferred call below the error handling, and the
tool cannot tell where that handling ends. With the error discarded there is
nothing to move the call past.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deferbeforeerr/cmd/deferbeforeerr
go tool deferbeforeerr ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deferbeforeerr ./...
```
