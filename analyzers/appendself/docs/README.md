# lesiw.io/tools/analyzers/appendself

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/appendself.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/appendself)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an append result assigned to a slice other
than the one it appends to.

```go
func grow(dst, src []int, v int) []int {
	dst = append(src, v) // append to src is assigned to dst; the result does not extend dst
	return dst
}
```

The spec defines the append built-in and its result
(https://go.dev/ref/spec#Appending_and_copying_slices). `append(src, v)` returns
a slice holding the elements of `src` followed by `v`. The assignment stores
that slice in `dst`. The elements `dst` held before the statement are absent
from the result.

The rule reports an append whose source the function does not read again after
the statement and whose destination the function does read again. A read after
the statement counts as a later read; a read inside the assignment does not.
`appendassign` reports the other case, where the source is read again and its
length does not change. A naked return counts as a read of every named result.

Report only: replacing `src` with `dst` changes the result from the elements of
`src` followed by `v` to the elements of `dst` followed by `v`. A local `src`
can also become unused, and the rewrite would not compile. The analyzer
reports without a fix.

Not reported, deliberately:

- append to the slice on the left side, which extends the destination;
- an append of two or more values;
- a spread append, which joins a second slice;
- a source that the function reads again after the statement, which
  `appendassign` reports;
- a source or destination that is not an identifier, as in
  `dst = append(src[:1], v)`, `p.items = append(src, v)`, or
  `m["k"] = append(src, v)`;
- a destination that is the blank identifier, `_ = append(src, v)`;
- a destination that is not a slice, as in `var dst any; dst = append(src, v)`;
- a destination that the function does not read again;
- a destination that the function writes again after the assignment, unless
  the write reads the destination, as `dst = dst[:0]` does;
- a source assigned by a range clause that contains the append; the source
  changes on every pass;
- a source and destination related by an earlier assignment in either
  direction, as in `src := dst[:1]` or `dst = src`, where the assigned
  expression mentions the other name; the two may share an array, so the
  append may extend the destination;
- a short declaration, `dst := append(src, v)` or
  `dst, n := append(src, v), 0`; the rule covers plain assignments to an
  existing destination;
- a local declaration of `append`, which shadows the builtin.

The rule is adapted from staticcheck's SA4021, which reports an append that
passes only the slice and replaces it with the slice. This rule covers an
append with one value, where the source and the destination differ, and reports
only because changing the source changes the result.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/appendself/cmd/appendself
go tool appendself ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -appendself ./...
```
