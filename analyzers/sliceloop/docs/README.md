# lesiw.io/tools/analyzers/sliceloop

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/sliceloop.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/sliceloop)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a loop that copies a source slice into
another slice one element at a time.

```go
func duplicate(dst, src []string) {
    for i, s := range src { // loop copies one element at a time
        dst[i] = s
    }
}
```

The section of the spec on appending to and copying slices defines `copy`,
which copies the elements of `src` into `dst` and returns the number copied
(https://go.dev/ref/spec#Appending_and_copying_slices). The loop ranges over
the source, or counts from zero to `len(src)`, and its body assigns one element
to the other slice at the same index. In the slice forms, both expressions
must be slices or type parameters over slices, with identical element types. A
counted loop may also copy a string into a byte slice, including through a type
parameter over strings and byte slices.

The loop usually takes one of these forms:

- a range loop with the key alone or with a blank value, as
  `for i := range src { dst[i] = src[i] }` or
  `for i, _ := range src { dst[i] = src[i] }`;
- a range loop with the key and the value, as
  `for i, s := range src { dst[i] = s }`;
- a three-clause loop from zero to `len(src)`, as
  `for i := 0; i < len(src); i++ { dst[i] = src[i] }`.

The source may also be a slice expression, a conversion, or a type assertion;
the target may be a slice expression, a conversion of a slice, or a type
assertion.

The counted form must test `i < len(src)` or `len(src) > i`; another exit test,
as `i <= len(src)-1`, is left alone. The post may be `i++`, `i += 1`,
`i = i + 1`, or `i = 1 + i`; a loop that increments in the body is left alone.

The rule matches a loop whose body is one assignment of the source element to
the target at the same index. The report does not depend on `copy` resolving to
the builtin; a loop in a scope where `copy` is shadowed is still reported. It
leaves alone:

- a loop whose key is assigned in the header instead of declared there with
  `:=`;
- a target or source expression that reads the loop key, the value, or the
  other expression;
- an expression that can run code when evaluated: an effectful call, a
  receive, or the address of a composite literal. The loop and copy can
  evaluate it a different number of times, and its effects can change the
  slices;
- a target expression that may allocate new storage each pass, as a slice or
  array literal or a conversion from a string;
- a loop whose source or target has an array type, a pointer to an array, or a
  type parameter over either; S1001 handles the array forms and this rule
  handles slices;
- a loop over a map, since a map is not a slice;
- a range loop over a string, since a range over a string yields the byte
  offset of each rune; copy would take every byte;
- a loop over a prefix of the source whose body indexes the base, as
  `for i := range src[:n] { dst[i] = src[i] }`, since the loop can read past
  the base length;
- a left shift in one slice, as
  `for i := range src[1:] { src[i] = src[1:][i] }`, because the target is the
  source's base; the mirror form is reported;
- a loop whose iteration count comes from the target, as
  `for i := range dst { dst[i] = src[i] }` or
  `for i := 0; i < len(dst); i++ { dst[i] = src[i] }`; the rule follows
  staticcheck's S1001 in matching a loop counted over the source;
- a loop whose target and source are the same expression;
- a body with more than one statement, or an assignment other than
  `dst[i] = src[i]` or `dst[i] = v`, as a body that writes two elements, reads
  another index, or tests the element.

Report only: `copy` stops at the shorter length, and the loop panics when the
target is shorter than the source. When the target starts after the source in
the same array, the loop can overwrite source elements it has not read yet;
the spec allows the source and destination to overlap, so `copy` can produce a
different result. A loop that runs zero times does not evaluate the target, and
`copy` does.

The rule is adapted from staticcheck's S1001. S1001 also reports loops over
arrays; this rule handles slices.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/sliceloop/cmd/sliceloop
go tool sliceloop ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -sliceloop ./...
```
