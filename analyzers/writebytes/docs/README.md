# lesiw.io/tools/analyzers/writebytes

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/writebytes.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/writebytes)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a byte slice converted to a string for
`io.WriteString`.

```go
type sink struct{}

func (sink) Write([]byte) (int, error) { return 0, nil }

func write(w sink, b []byte) {
    io.WriteString(w, string(b))
}
```

`io.WriteString`'s doc says that it invokes a `StringWriter`'s `WriteString`
and calls `Write` "exactly once" otherwise (https://pkg.go.dev/io#WriteString).
Converting the slice to a string first copies every byte, so `w.Write(b)`
writes the same bytes without the copy when `Write` is the method that runs.
The rewrite is valid because `io.Writer`'s doc requires that `Write` not modify
or retain the slice (https://pkg.go.dev/io#Writer). The fix is offered only
when `b` cannot be nil, since a nil `b` passed to `Write` may differ from the
non-nil empty slice `io.WriteString` passes.

The call is reported when the second argument is `string(b)`, `b` is assignable
to `[]byte`, and the writer is concrete and does not implement
`io.StringWriter`. A conversion from a string, a slice of runes, an integer, or
a slice whose elements are not exactly `byte` is left alone, as is a call to a
local `WriteString`. The conversion may be parenthesized, and the file may give
`io` another name or import it with a dot. An untyped `nil` writer is left
alone, as is a call through a function value assigned from `io.WriteString`.

The fix replaces the call with `w.Write(b)`. It is withheld in these cases:

- a comment sits in the text it would replace;
- the call is the direct call of a `defer` or `go` statement, where the string
  conversion copies `b` before the call runs;
- the operand may be nil;
- the fixes together would leave a dot import unused.

The diagnostic is still reported when the fix is withheld.

Fixable: the call becomes `w.Write(b)` when the operand cannot be nil.

The rule is adapted from staticcheck's SA6006, which reports the call for any
writer and offers no fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/writebytes/cmd/writebytes
go tool writebytes ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -writebytes ./...
go tool lesiw.io/tools/cmd/fix -writebytes ./...
```
