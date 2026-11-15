# lesiw.io/tools/analyzers/badsorting

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/badsorting.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/badsorting)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `sort.Slice` call whose less function
does not order the elements by the indexes it was given.

```go
func byPositions(rows []row) {
	sort.Slice(rows, func(i, j int) bool { return rows[0].rank < rows[1].rank }) // the less function does not order rows by the indexes it was given
}
```

`Less` takes two element indexes and says what they mean: "Less reports whether
the element with index i must sort before the element with index j." A
comparison at a constant index reads the same slot every time, whichever element
the sort has moved into it, so it no longer describes the pair the less function
was given. `rows[i].rank < rows[i].rank` compares one element with itself and
returns false for every pair, and `rows[0].rank < rows[1].rank` compares
whatever sits at slots 0 and 1.

The call must be `sort.Slice`, `sort.SliceStable` or `sort.SliceIsSorted`, the
three that take a less function over element indexes. `slices.SortFunc` gives
its comparison two elements, so there are no indexes to misread. A `Less` method
on a `sort.Interface` type carries the same contract, and the rule does not read
it.

A less function that is not a func literal is left alone, since the analyzer
cannot see what it reads.

A comparison that does not mention the slice is left alone: a `return true`, an
`i < j`, and a comparison over a parallel `keys[i] < keys[j]` are reported by
the `sortmismatch` analyzer when the whole body names the slice nowhere and
holds no call. That rule reads whole bodies, so a body that names the slice in
one return and returns something else in another is read by neither analyzer.
A comparison that only calls a helper, `return less(i, j)`, is left alone by
both. A
comparison of one expression with itself, `rows[i].rank < rows[i].rank`, is
also reported by `dupsubexpr`.

Beyond naming the slice, the comparison must read one value at two different
indexes, and neither may be constant: `rows[i].rank < rows[j].rank` and
`rows[j].rank < rows[i].rank` both do, `rows[i].rank < rows[i].rank` does not,
and neither does `rows[0].rank < rows[1].rank`. A comparison that reads two
dynamic indexes is accepted, whatever else it reads, as in
`rows[i].rank < rows[j].rank || rows[0].key == ""`.

Index expressions are compared after their parentheses are removed, and the
rule does not read the less function's parameter names. Any two dynamic indexes
of one value count, and the value need not be the sorted slice, though the
comparison must mention the slice somewhere:
`len(rows) > 0 && q[i].rank < q[j].rank` is accepted, whatever `q` is. Names
are compared, not values, so one name at `i` and another name for the same
slice at `j` is a report: `q := rows` before `rows[i].rank < q[j].rank`.
`x, y := i, j` before
`rows[x].rank < rows[y].rank` is accepted, and so are
`rows[i].rank < rows[k].rank` and `rows[i].rank < rows[j+1].rank`. One index of
each of two values is a report: `rows[i].rank < other[j].rank`.

A call that is handed the slice reads elements the comparison does not name, so
the comparison is left alone: `rows.at(i) < rows.at(j)` reads the elements at
`i` and `j` inside the method. A call on an element reads only that element, so
`rows[i].label() < rows[0].label()` compares element `i` with a fixed one and is
reported, and `rows[i].label() < rows[j].label()` compares the two elements it
was given and is left alone. A conversion and a builtin that only reads its
arguments count as no call at all, so `len(rows) < 0` reads no position.

A slice expression is read as its base: element `i` of `rows[:2]` is `rows[i]`,
and a low bound counts as a position, so `rows[i:][0]` is `rows[i]`. The rule
does not track offsets: a comparison over `rows` is accepted for `rows[2:]`,
though a comparison of that tail reads `rows[i+2]` and `rows[j+2]`.

A conversion is read as the value it converts, whatever the element types are,
and an address as the value it points at, so `[]item(rows)[i]` is `rows[i]`,
`(*ps)[i]` is a position of `*ps`, and a helper handed `&rows` is handed the
slice. A base that is a call or a type
assertion is never matched, so a comparison over one is left alone.

The body is read one return at a time, so a comparison in a branch is judged on
its own:
`if rows[i].rank != rows[j].rank { return rows[i].rank < rows[0].rank }` is
reported even though the return after it reads two indexes, and a body with
several returns points at the one that is wrong. The rule reads only the
expression a return carries, so `less := rows[0].rank < rows[1].rank` followed
by `return less` is not read, and neither is a body that assigns a named result
and returns bare. A return inside
a nested function literal is not read.

The analyzer resolves the callee, so a dot import works and a variable holding
the function (`f := sort.Slice`) does not.

The rule is inspired by go-critic's `sortSlice`, which reads the less function
of `sort.Slice` and reports the comparison that does not use the sorted slice.
The name comes from go-critic's `badSorting`, which reports
`xs = sort.StringSlice(xs)`.

Report only: `rows[0]` may have been meant as `rows[i]`, and the comparison does
not show which index each position was meant to be.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/badsorting/cmd/badsorting
go tool badsorting ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -badsorting ./...
```
