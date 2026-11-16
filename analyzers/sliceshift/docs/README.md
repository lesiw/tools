# lesiw.io/tools/analyzers/sliceshift

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/sliceshift.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/sliceshift)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a loop that shifts elements within one
slice by an offset.

```go
func shift(xs []int, n int) {
    for i := 0; i < n; i++ {
        xs[i] = xs[i+1]
    }
}
```

The "Appending to and copying slices" section of the spec says that `copy`
copies the elements of the source into the destination and that "[t]he result
is independent of whether the memory referenced by the arguments overlaps"
(https://go.dev/ref/spec#Appending_and_copying_slices). The loop reads and
writes one slice at neighboring indexes, and copy does the same shift in one
call.

The loop's init statement declares its index, its condition tests the index
against a bound with a strict comparison, and its post statement moves the
index by one. The index may be on either side of the comparison. The body is
one assignment between two elements of the same slice. The destination index is
the loop index, and the source index is that index plus an offset, in either
order, or minus an offset for the reverse direction. The offset is an
identifier or a literal.

A forward loop reads the element ahead of the one it writes, and a reverse
loop reads the element behind it. In both directions a read reaches an element
the loop has not written yet, so `copy` gives the same result. A forward loop
with a non-constant offset is reported only when its index starts at zero; a
negative offset makes the first read negative, and the assignment evaluates its
indexes before it writes
(https://go.dev/ref/spec#Assignment_statements), so the loop panics first. The
bounds and the offset must not read an element of the slice, since a write
could change them while the loop runs. A scope that declares `copy` is left
alone, because the replacement would not call the builtin.

## Checks

The loop is a `for` statement whose init statement declares the index with
`:=`, whose condition tests that index with one strict comparison, and whose
post statement is `i++` or `i--`. The body is one plain assignment between two
index expressions. The destination index is the loop's index and the source
index is that index plus or minus an offset. Both index expressions use the
same slice identifier, and its type is a slice. The loop's destination element
becomes the destination of copy, and its source element becomes the source.

The offset is an identifier or a literal, in either order for a forward loop.
A reverse loop subtracts a positive constant offset. A loop with constant
bounds is reported only when it runs at least once.

Not reported, deliberately:

- a loop that copies between two slices; this rule covers one slice;
- an array, a map, or a type parameter, which this rule leaves alone;
- a source offset that reads the same slice, as in `xs[i+xs[0]]`, or a bound
  that does, as in `xs[0]`; a write can change the element;
- a loop with another statement in its body, with `<=`, with `+=`, with an
  assigned index, or with a destination index other than the loop's index;
- a bound that mentions the index, as in `i+1 < len(xs)`;
- a forward loop with a non-constant offset whose index does not start at
  zero;
- a reverse loop whose offset is not a positive constant;
- a scope that declares `copy`, because the rewrite would call that
  declaration instead of the builtin.

## Fixes

The fix replaces the loop with `copy` when every index the loop visits is
within the slice's length. That holds for a forward loop that starts at a
non-negative constant and stops at `len(xs)-k` with `k` a constant at least
the offset, and for a reverse loop that starts at `len(xs)-1-c` with `c` a
constant over a stop of at least the offset minus one, and the offset is a
positive constant. The loop's condition becomes a guard, so the copy is skipped
when the loop would not run. For example:

    for i := 0; i < len(xs)-1; i++ {
        xs[i] = xs[i+1]
    }

becomes:

    if 0 < len(xs)-1 {
        copy(xs[:len(xs)-1], xs[1:len(xs)])
    }

Any other bound is reported without a fix. A `copy` over slices whose bounds
are not known to be within the slice's length would change where the loop
panics (https://go.dev/ref/spec#Index_expressions) and which elements it wrote
first. The fix is also withheld when `start+offset`, `stop+1`, or a bound
derived from the subtraction chain would exceed the target's largest `int`,
since a constant expression that overflows does not compile
(https://go.dev/ref/spec#Constant_expressions). It is withheld when a
comment sits inside the loop, since the edit would delete it, when an
expression has no one-line printed form, when the loop does not begin its line,
and when the replacement would run past the width `linelen` measures. The
diagnostic is still reported.

The rule is adapted from staticcheck's S1018, which reports the forward loop
that starts at zero. This rule also reports the reverse loop and the wider
operand and bound forms.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/sliceshift/cmd/sliceshift
go tool sliceshift ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -sliceshift ./...
go tool lesiw.io/tools/cmd/fix -sliceshift ./...
```
