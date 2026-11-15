# lesiw.io/tools/analyzers/sliceclear

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/sliceclear.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/sliceclear)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a loop that zeroes every element of a
slice.

```go
func reset(xs []int) {
    for i := range xs { // loop can be replaced by clear(xs)
        xs[i] = 0
    }
}
```

The spec says `clear` "sets all elements up to the length of s to the zero
value of T".

The loop must declare its index, and its body must be exactly one assignment:
the element's zero value, written to the element at that index. The range, the
bound, and the assignment must name the same slice. Identifiers, field names,
indexes, literals, and dereferences are compared; a call or a slice expression
is not, so a loop over `xs[:n]` is left alone. A range may count the elements
through the predeclared `len` instead of naming the slice, and the slice may be
a type parameter constrained by a union of slice types, as in `[S ~[]int]`.

That zero value is a zero constant, `nil`, or an empty literal of a struct or
array type. A constant is not the zero value of an interface element, where
`clear` writes nil. A constant is the zero value of a type-parameter element,
because the assignment compiles only if every type in the set accepts it.

A counted loop starts its index at zero, steps it by one, and compares it to the
length with `<`. Its bound may not read its own index, since the call cannot
name it.

The rule does not report a loop over a map, whose entries `clear` deletes rather
than zeroes, or over an array, which `clear` does not take. It does not report a
body that writes through the element, such as `*xs[i] = 0` or `xs[i].n = 0`,
because `clear` zeroes the element itself.

A file whose Go version is below 1.21 is left alone, because `clear` is not in
the language at that version. A loop in a scope that declares `clear` is left
alone too, since the call would not build.

From Go 1.22, `modernize`'s `rangeint` rewrites a counted loop to the range
form, which this rule then clears. Both report the loop, and the driver applies
one edit because the two edits overlap.

The rule is adapted from go-critic's `sliceClear`, which reports the counted
loop; its documentation gives the range form as the replacement. x/tools has an
open issue for a `modernize` check that replaces these loops with `clear`
([issue 82956](https://go.dev/issue/82956)).

Fixable: the loop becomes `clear(xs)`. The fix is withheld when a comment sits
inside the loop, since the edit would delete it.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/sliceclear/cmd/sliceclear
go tool sliceclear ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -sliceclear ./...
go tool lesiw.io/tools/cmd/fix -sliceclear ./...
```
