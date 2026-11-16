# lesiw.io/tools/analyzers/appendloop

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/appendloop.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/appendloop)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a range loop that appends each element of
a slice by index, one at a time.

```go
func merge(dst, src []string) []string {
    for i := range src { // loop appends one element at a time
        dst = append(dst, src[i])
    }
    return dst
}
```

The section of the spec on appending to and copying slices specifies `append`,
including `append(dst, src...)`, which appends every element of `src` in one
call. Each
append may grow and copy the slice; one append with the whole slice grows it
once.

The range clause must use `:=` for its key. Its body is either
`dst = append(dst, src[i])` or `v := src[i]` followed by
`dst = append(dst, v)`. The target is a name, a field path, or a dereference of
one, read the same way in both places. A body that tests the element, appends
two values, or writes anywhere else does not have a spread form. A loop that
assigns the key, or the value local in the two-statement form, instead of
declaring it is left alone, since a spread append would drop the write to the
variable. In the two-statement form the target cannot contain the value, since
each pass binds it to a different element. `rangeappendall` covers the value
form.

The source must be assignable to a slice of the target's element type, so a
loop that widens each element is left alone. A target or source whose type set
contains one slice type counts as a slice, and the element type comes from the
type set. A loop over a map or an array is left alone, since neither is
assignable to a slice. A string source is left alone: a range over a string
visits the start of each rune, so indexing it does not visit every byte. A
target whose path contains a call or an index is left alone.

An index loop whose range expression contains a call is left alone: the call
may have an effect, and the analyzer does not compare call expressions. The
loop must append `src[i]`, where `src` is the range expression and `i` is the
loop key. A bound in a slice expression must read the same value in both
places; a call or an arithmetic bound is left alone. A type assertion, a
conversion, or a composite literal is left alone as well.

The source and the target are usually parameters, and a caller can pass two
slices over the same array. An append to the target can overwrite an element
the source has not read yet. The spec says the result of append is independent
of whether the memory referenced by the arguments overlaps, so the loop and the
call can produce different results. A source that is a different expression
containing the target is left alone, since a bound such as `cap(dst)` can reach
past the target's length.

A loop over the target itself reads only elements before its captured length,
so it carries a fix unless a comment inside the loop would be deleted. The
spread append may grow once instead of once per element, so the final capacity
can differ. The other loops are report-only, since the target and the source
are usually parameters and a caller can pass overlapping slices.

The index forms are adapted from staticcheck's `S1011`.

Fixable for a loop over the target; report only otherwise.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/appendloop/cmd/appendloop
go tool appendloop ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -appendloop ./...
go tool lesiw.io/tools/cmd/fix -appendloop ./...
```
