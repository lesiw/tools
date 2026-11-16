# lesiw.io/tools/analyzers/errdiscard

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errdiscard.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errdiscard)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call whose only result is an error,
written as a statement.

```go
func remove(path string) {
    os.Remove(path) // os.Remove discards its error
}
```

Go's [Code Review
Comments](https://go.dev/wiki/CodeReviewComments#handle-errors) say: "Do not
discard errors using `_` variables. If a function returns an error, check it to
make sure the function succeeded. Handle the error, return it, or, in truly
exceptional situations, panic."

The error is the only result, so a statement that calls the function discards
the only report of failure.

The statement is reported when the call returns exactly one value and that
value's type implements `error`, so a call returning a named error type is
reported as well. A call through an interface is judged by the interface's
method, so `w.WriteByte(c)` on an `io.ByteWriter` is reported: an
implementation the analyzer cannot see can fail. A type parameter is judged
through the methods its constraint declares; `go/types` adds none from a type
term, so a call whose result is a parameter is reported when the constraint
declares `Error`. A call that instantiates the parameter returns the concrete
type, and the rule reads that type.

Not reported:

- a call that returns a value beside the error;
- a call in a `defer` or `go` statement, which is not an expression statement.
  A call inside the function literal of one is an expression statement too, and
  is reported;
- a call assigned to the blank identifier, as in `_ = os.Remove(path)`, which
  states that the discard is deliberate, though the Code Review Comments ask for
  it to be handled;
- a call to a standard library method documented as never returning an error:
  `(*bytes.Buffer).WriteByte`, `(*hash/maphash.Hash).WriteByte`,
  `(*io.PipeReader).CloseWithError`, `(*io.PipeWriter).CloseWithError`, and
  `(*strings.Builder).WriteByte`. The exemption names the receiver type,
  because a method of the same name on another type can fail.

An `//ignore` directive names the analyzers it covers, so a `//ignore:errcheck`
above a deliberate discard does not cover this rule. Add `errdiscard` to the
directive, or drop the name to cover every analyzer.

The rule is inspired by gosec's `G104`. The suite's `errcheck` reports a call
whose result holds an error, and reads the type only when it is a named type, a
pointer, or a tuple, so a type parameter, an alias of `error`, and an unnamed
interface are the type shapes this rule adds. `discardedpure` reports a pure
callee whose result is dropped, so one statement can carry more than one
message.

Report only: the repair is the handling, which changes behavior and is the
caller's code to write. A report the author means to keep is suppressed with an
`//ignore` directive.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errdiscard/cmd/errdiscard
go tool errdiscard ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errdiscard ./...
```
