# lesiw.io/tools/analyzers/stringscount

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/stringscount.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/stringscount)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a loop that counts the elements of a
slice with a counter.

```go
func count(xs []string) int {
    n := 0
    for range xs { // loop can be replaced by len(xs)
        n++
    }
    return n
}
```

The spec says a range over a slice produces iteration values from index 0 up to
`len(a)-1`, and `len` returns the length of a slice. A loop that adds one to a
counter on every pass leaves the counter at `len(slice)`.

The counter is an `int` identifier initialized to zero by the statement just
before the loop. That statement may be `n := 0`, `n = 0`, `var n int`, or
`var n = 0`. The body must be exactly one `n++`. A counter with another
starting value or another type is left alone, and so is a body with any other
statement. An assignment that sets the counter to zero is accepted only when
the counter appears once on the left-hand side and every left-hand side is a
name. A counter that appears twice, or a left-hand side that is not a name, is
left alone.

A range loop must have no iteration variables or only blank ones, and its
expression must be a slice. A counted loop must have the form
`for i := 0; i < len(slice); i++`. Its index must be declared in the loop, and
the slice expression must not contain it. In a counted loop the bound must not
contain the counter or a `*p` expression, since the condition runs again on
every pass. A counted loop whose index is also the counter is left alone, since
the loop does not count every element.

The slice expression must not contain a call other than a pure builtin or a
conversion, a channel receive, or the address of a composite literal. A range
expression is evaluated once. A counted loop evaluates its bound on every pass,
and the replacement evaluates it once.

The replacement calls the builtin `len`, so a loop in a scope that declares
`len` is left alone.

A loop over a map, a string, an array, a channel, an integer, or a function is
left alone, as is a loop over a type parameter. A labeled loop is left alone,
since a `goto` can enter it without the initialization.

The rule is adapted from the loop-to-call shape of staticcheck's S1011, which
replaces a range-append loop with append. Here the call is `len`.

Fixable: the loop becomes `n = len(slice)`. When the initialization is a
separate declaration of only the counter, the replacement covers both
statements. An assignment that resets the counter stays in place. In a range
loop, the initialization stays when the slice expression contains the counter
or a `*p` expression. Folding a declaration would put the slice expression
before the counter's declaration. When a comment lies between the initialization
and the loop, the fix leaves the initialization in place, since folding would
delete the comment. No fix is offered when a comment sits inside the loop,
since the edit would delete it.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/stringscount/cmd/stringscount
go tool stringscount ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -stringscount ./...
go tool lesiw.io/tools/cmd/fix -stringscount ./...
```
