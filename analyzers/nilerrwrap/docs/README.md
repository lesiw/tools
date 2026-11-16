# lesiw.io/tools/analyzers/nilerrwrap

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilerrwrap.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilerrwrap)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `fmt.Errorf` call whose `%w` verb has no
error to wrap.

```go
func load() error {
    return fmt.Errorf("read: %w", nil)
}
```

The `errors` package documentation states that a nil error returned from
`Unwrap` means the error wraps nothing. The `fmt.Errorf` documentation says it
is invalid to supply `%w` with an operand that does not implement `error`. A
`%w` argument that is missing, is not an error, or is nil has no cause to wrap:
fmt writes a `%!w(...)` marker for the argument or `%!w(MISSING)` for a missing
one.

The rule is inspired by the x/tools printf analyzer, which reports the missing
and non-error operands; this rule adds the nil error. The overlap is
intentional: a run of this analyzer alone covers all three cases. In the suite,
printf reports the missing and non-error operands too, so a call that both
analyzers report has two diagnostics. The format string has to be constant, so
the verbs are known.

A call with a spread argument is left alone, because the argument at a `%w`
position is not known. A call with an indexed argument or a star width or
precision is left alone, because the analyzer does not resolve an explicit
index or a star argument. A nil literal and a conversion of nil to an interface
type are reported; any other expression is not inspected for nilness. An
interface-typed argument is left alone except for a conversion from nil or from
a concrete type that does not implement `error`; the dynamic type of any other
interface value is not known. A type parameter is left alone; the analyzer does
not inspect its type set. A call through a function value or a wrapper is left
alone, because the callee expression is not `fmt.Errorf`.

Report only: the tool cannot invent the error the author meant to wrap.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilerrwrap/cmd/nilerrwrap
go tool nilerrwrap ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilerrwrap ./...
go tool lesiw.io/tools/cmd/fix -nilerrwrap ./...
```
