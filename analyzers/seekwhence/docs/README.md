# lesiw.io/tools/analyzers/seekwhence

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/seekwhence.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/seekwhence)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `Seek` call with an `io.Seek*` constant
in the offset argument.

```go
func toEnd(f *os.File) {
    f.Seek(io.SeekEnd, 0) // the first argument of Seek is the offset; SeekEnd belongs in the second
}
```

`io.Seeker`'s documentation says `Seek` "sets the offset for the next Read or
Write to offset, interpreted according to whence". The first argument is the
offset and the second is the whence, so a call whose first argument is
`io.SeekStart`, `io.SeekCurrent`, or `io.SeekEnd` usually has them in the
other order. A whence constant is also a valid numeric offset: `io.SeekStart`
is `0`, so an offset of zero can be written as `0`.

The rule reads a method call with the signature of `io.Seeker.Seek`. That
includes a concrete type such as `*os.File` and an interface the author
declares with the method. A constant declared outside package `io` is not
`io.SeekStart`, whether it is named `SeekStart` or defined as `io.SeekStart`,
and a method with a different signature is not the method `io.Seeker`
declares, so neither is reported. A method expression is left alone because
its receiver is the first argument. A function named `Seek`, a field of
function type named `Seek`, and a method value stored in a variable and called
later are not method calls, so they are left alone too. The first argument
must be the constant itself, with or without parentheses; a conversion or an
arithmetic expression around it is left alone.

Where the second argument may be an offset, the rule reports without a fix: a
swap changes the seek target, and writing the first constant as a literal
would only hide the suspected mistake. Writing `0`, `1`, or `2` in the first
argument keeps the numeric offset and clears the report.

Where the second argument contains an `io.Seek*` constant, the fix writes the
first constant's value as a literal in the offset argument, which preserves
behavior. The fix is withheld when a comment inside the first argument would be
deleted and when the constant in the first argument is dot-imported, since
replacing it may leave the dot import unused. In particular, no fix is offered
when the second argument is a non-literal constant with the value `0`, `1`, or
`2` that contains no `io.Seek*` constant, since that value may be the offset.
The diagnostic names the value.

The rule is adapted from staticcheck's `SA1013`, which swaps the arguments.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/seekwhence/cmd/seekwhence
go tool seekwhence ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -seekwhence ./...
go tool lesiw.io/tools/cmd/fix -seekwhence ./...
```
