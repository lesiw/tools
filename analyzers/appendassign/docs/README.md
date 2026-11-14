# lesiw.io/tools/analyzers/appendassign

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/appendassign.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/appendassign)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an append result assigned to another slice
that the function still reads.

```go
func f(xs, ys []int, v int) ([]int, []int) {
	ys = append(xs, v) // append to xs is assigned to ys, so xs keeps its length
	return xs, ys
}
```

append returns a slice one element longer, and the new element is written into
an array the first argument may already share. The Go blog says that "since the
slice header is always updated by a call to append, you need to save the
returned slice after the call", and writes the idiom as
`slice = append(slice, 4)`. A left side that names another slice leaves the
first argument at its old length while the result is one longer.

The rule matches the left side against the first argument of append, for
`ys = append(xs, v)` or its tuple form `n, ys = 0, append(xs, v)`, and reports
the pair only when the function reads the first argument after it.

Silent, deliberately:

- a first argument the function does not read again, because nothing then shows
  that the append went elsewhere. A read that the assignment follows in the
  source text, such as the `xs` of `for range xs` above an append in the loop
  body, is not seen;
- a write to the first argument, as `xs = nil` after the append;
- an assignment inside a func literal, which is measured against that literal
  alone;
- a declaration of the result, `ys := append(xs, v)`, because a new name is how
  a copy is written;
- a spread append, which joins a second slice to the first
  (`ys = append(xs, more...)`) or puts the first in front of the second
  (`xs = append(ys, xs...)`);
- a first argument that is not an identifier, `ys = append(xs[:1], v)`, or a
  left side that takes no slice, as `m["k"] = append(xs, v)` or
  `_ = append(xs, v)`: a prefix, a map entry, or a discarded result does not
  name the slice that came in.

The rule is adapted from go-critic's appendAssign, which reports the assignment
without the later read and takes a field on either side.

Report only: neither `xs = append(xs, v)` nor `ys = append(ys, v)` preserves
the behavior, so no fix is offered.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/appendassign/cmd/appendassign
go tool appendassign ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -appendassign ./...
```
