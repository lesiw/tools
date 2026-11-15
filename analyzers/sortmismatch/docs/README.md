# lesiw.io/tools/analyzers/sortmismatch

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/sortmismatch.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/sortmismatch)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `sort.Slice` call whose less function
does not name the slice.

```go
func byKeys(rows []row, keys []string) {
    sort.Slice(rows, func(i, j int) bool { return keys[i] < keys[j] }) // the less function does not name rows
}
```

`Less` takes two element indexes and says what they mean: "Less reports whether
the element with index i must sort before the element with index j." A
comparison over another slice orders the elements by values the call does not
reorder, so the elements and the values that ordered them no longer line up. A
comparison that names no element orders the elements by nothing.

The call must be `sort.Slice`, `sort.SliceStable` or `sort.SliceIsSorted`, the
three that take a less function over element indexes. `slices.SortFunc` takes
the elements themselves, where a comparison cannot name the wrong slice.

The second argument must be a func literal. A comparison that is not one is left
alone, since the analyzer cannot see which slice it names.

The comparison must hold no call, since a call may read the slice without naming
it. A comparison built on `cmp.Compare` is left alone for that reason.

The comparison names the slice in any form. `rows[i].rank < rows[j].rank`,
`*ps[i] < *ps[j]`, and `b.items[i] < b.items[j]` all name the slice that is
sorted. Any mention counts, so a comparison that reads one side of the slice,
`rows[i].rank < other[j].rank`, and one that reads a fixed pair,
`rows[0].rank < rows[1].rank`, are left alone.

A slice expression is read as its base: element `i` of `rows[:2]` is `rows[i]`,
and element `i` of `rows[2:]` is `rows[i+2]`. The rule does not track offsets, so
a comparison over `rows` is accepted for `rows[2:]` even when it should add the
two.

The base must be a name the comparison can write: an identifier, a field, an
element, or a dereference. A type assertion, a call, and a literal are left
alone, since the comparison has no expression for what they return.

The rule compares names, not values, so a comparison over another name for the
same slice is reported: `sl := rows` followed by a comparison over `sl`. Writing
`sort.Slice(sl, ...)` clears the report.

The analyzer resolves the callee, so a dot import works and a variable holding
the function (`f := sort.Slice`) does not.

The rule is adapted from go-critic's `sortSlice`, which reads `sort.Slice` and
`sort.SliceStable` and reports a body that returns one comparison of the wrong
values. This rule reads `sort.SliceIsSorted` too and reads the whole body, so the
order a comparison indexes in takes no part in the report.

Report only: the author may have meant `sort.Slice(keys, ...)`, or a comparison
over `rows`. There is no comparison that orders `rows` by a parallel `keys`,
since the sort swaps only `rows`: the key has to travel with the element,
carried on it or looked up by it (`keys[rows[i].id]`).

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/sortmismatch/cmd/sortmismatch
go tool sortmismatch ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -sortmismatch ./...
```
