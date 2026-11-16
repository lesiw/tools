# lesiw.io/tools/analyzers/copyto

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/copyto.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/copyto)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a loop that copies one slice into another
element by element.

```go
func copyAll(dst, src []int) {
    for i, v := range src { // loop copies src into dst element by element; use copy(dst, src), which writes at most the destination's length and allows overlap
        dst[i] = v
    }
}
```

The "Appending to and copying slices" section of the spec says that `copy`
copies the elements of the source into the destination and returns the number
copied (https://go.dev/ref/spec#Appending_and_copying_slices). The loop writes
each element at the index it reads, and copy writes the elements in one call.

The rule is adapted from staticcheck's S1001, which replaces a copying loop
with `copy` or an array assignment.

## Checks

The loop must declare its index and copy every element the source holds. Four
forms are reported:

- a range that reads the index and the value, `for i, v := range src`;
- a range that reads the index and the element, `for i := range src` or
  `for i, _ := range src`;
- a range over the element count, `for i := range len(src)`;
- a counted loop from zero to `len(src)`,
  `for i := 0; i < len(src); i++`.

A range over an integer visits zero through `len(src)-1` (Go 1.22,
https://go.dev/ref/spec#For_statements), so `for i := range len(src)` copies
the same elements as a range over `src`.

The body is exactly one assignment to an element of the destination, at the
index the loop declares. The source and destination must be slices with
identical element types. Each can be an identifier, a field, a dereference, an
index, a slice expression, a conversion, or a type assertion. A composite
literal can be a source only in the value form. The destination must also be free of side
effects and keep the same slice in every form, so a call with effects or a
composite literal is left alone. The index and counted forms require the
source to keep the same slice after a pass writes the destination; they leave
a composite literal alone because it allocates on each evaluation. In those
forms, the body must read the source at the loop index, and every name in that
read must resolve to the same object as in the loop's source expression. The
fix prints the expressions and drops parentheses around the whole expression.
Neither expression may mention the loop's variables, and a range over `next()`
is left alone.

The value form evaluates the range expression once
(https://go.dev/ref/spec#For_statements), so a source there may read the
destination. The index form reads the source in the body on each pass, and the
counted form reads it in the condition and the body; both need the source to
keep the same slice.

A range whose key and value are already declared, `for i, v = range src`, is
left alone: the loop assigns those variables, and replacing it with `copy`
removes the assignments. A scope that declares `copy` is left alone, because
the fix would not call the builtin. A loop whose `len` call resolves to a
declaration other than the builtin is left alone.

Not reported, deliberately:

- a destination or source that is an array, where `copy` takes slices;
- a destination whose element type differs from the source's, as in `[]any`
  and `[]int`, where an element of the source is assignable to the
  destination's element type but `copy` requires identical element types;
- a type parameter whose type set holds slices, which this rule does not read;
- a loop with another statement in its body, with an index other than the one
  the loop declares, or with a source other than the range expression or the
  operand of its `len`;
- a counted loop that starts at anything other than zero, ends at
  `<= len(src)`, or steps by anything but `++`;
- a string source, including a counted loop into a byte slice, where `copy`
  has a special case; the rule covers loops between slices;
- in the index and counted forms, a source whose index or slice bound reads
  an element, as in `src[:dst[0]]` or `rows[other[0]]`, and in every form a
  destination whose index or slice bound reads an element, as in
  `dst[dst[0][0]]`; an earlier pass can change that element;
- in the index and counted forms, a source that reads an element below its
  root, as in `nodes[0].f`, and in every form a destination that does; only a
  root index such as `rows[k]` may read an element;
- in the index and counted forms, a source that reads through a pointer, as in
  `p.children` or `*p`; the pointer can point into the destination;
- a destination that reads through a pointer whose pointee is the
  destination's element type or a field of it, as in `p.children` for
  `[]node`; a write can replace the pointee;
- a destination whose index or slice bound reads through a pointer, as in
  `dst[c.n:]`; the pointer can point into the destination;
- a destination that reaches a field through an embedded pointer, as in
  `h.items` for a holder that embeds `*inner`; the rule leaves the promoted
  selector alone;
- in the index and counted forms, a source that is an element of the slice
  the destination indexes, as in `dst[0]` for a recursive slice type, and in
  every form a destination that is such an element, as in `dst[0][i]`; a pass
  can replace the element that holds the slice.

## Fixes

The fix replaces the loop with `copy(dst, src)`. It is withheld when a comment
sits inside the loop, since the edit would delete the comment, when a printed
expression contains a newline, since the call is written on one, when the loop
does not begin its line, since the fix is written for the loop's line, and
when the replacement would run past the width `linelen` measures. The
diagnostic is still reported.

When the destination is not an identifier, the fix checks the source length
first, so the destination is evaluated only when the loop runs:

    if len(src) > 0 {
        copy(p.dst, src)
    }

The length check evaluates the source twice, so the fix is withheld when the
source holds a call or a composite literal, and a guarded fix is withheld when
`len` resolves to another declaration. The diagnostic is still reported. When
an expression has no one-line printed form, the message names neither
expression and leaves out the call.

The loop writes the source's elements into the destination. When the
destination is shorter, the loop panics on the first index past its length,
and `copy` writes at most the destination's length. When the slices overlap,
the forward loop can read an element it has already written, and `copy` is
defined for overlapping slices. The fix changes the program in both cases.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/copyto/cmd/copyto
go tool copyto ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -copyto ./...
go tool lesiw.io/tools/cmd/fix -copyto ./...
```
