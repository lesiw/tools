# lesiw.io/tools/analyzers/unconvert

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unconvert.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unconvert)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a conversion to a type its operand already
has.

```go
func f(n int, b []byte) (int, []byte) {
    return int(n), []byte(b) // unnecessary conversion
}
```

The specification's "Conversions" section says an explicit conversion is an
expression of the form `T(x)`, where `T` is a type and `x` is an expression
that can be converted to `T`. "Assignability" makes a value assignable to a
variable of its type, so `int(n)` for an `int` n and `[]byte(b)` for a
`[]byte` b change nothing.

A conversion to a floating-point or complex type is left alone. The
specification's "Floating-point operators" section says an explicit
floating-point conversion rounds to the target precision and prevents a fused
operation, so dropping one can change the result even when the operand already
has that type. A type parameter whose type set is not known to exclude such a
type is left alone too. The tool approximates that intersection: a type
parameter is left alone when every embedded element admits a floating-point or
complex type, even when the intersection admits no such type.

An untyped expression has no concrete type, so a conversion of one is left
alone: `int(1)` gives the literal a type. A typed constant is reported as any
other typed value. A comparison yields an untyped boolean, and the predeclared
nil is untyped as well. A shift whose left operand is written as an untyped
constant is left alone because the conversion may supply the type that operand
would assume. A typed left operand makes the shift typed, so a conversion that
repeats that type is reported.

A conversion to a different type is not reported, even when it is lossless:
`int64(n)` for an `int` n changes the type.

A generated file is left alone, since an edit would change generated text
instead of its source. The `// Code generated ... DO NOT EDIT.` marker counts,
as do a name ending in `.cgo1.go` or `-d`, and the names `_cgo_gotypes.go` and
`_cgo_import.go`.

Fixable: the fix removes the conversion. Parentheses belong around an operand
where regrouping would change the value or the parse, and around an operand
whose first token would join the operator before it. `int(a+b) * c` becomes
`(a + b) * c`, `c - int(a+b)` becomes `c - (a + b)`, and `switch item(item{})`
becomes `switch (item{})`. An unformatted `c-int(-n)` becomes `c - (-n)`.

A comment in the dropped text withholds the fix, since the edit would delete
the comment. An inner conversion gets no fix while an enclosing conversion has
a fix, so the fix is applied to the outermost conversion first. When a comment
in the text an outer fix removes withholds that fix, the inner conversion
keeps its fix.

The rule is adapted from mdempsky's unconvert.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unconvert/cmd/unconvert
go tool unconvert ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unconvert ./...
go tool lesiw.io/tools/cmd/fix -unconvert ./...
```
