# lesiw.io/tools/analyzers/rangeappendall

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/rangeappendall.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/rangeappendall)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a range loop that appends each element of a
slice one at a time.

```go
func f(dst, src []string) []string {
	for _, s := range src { // loop can be replaced by append(dst, src...)
		dst = append(dst, s)
	}
	return dst
}
```

The section of the spec on appending to and copying slices defines
`append(dst, src...)`, which appends every element of `src` in one call. Each
append may grow and copy the slice; one append with the whole slice grows it
once.

The loop must declare its value, and its body must be exactly one call,
`append(dst, value)` assigned to `dst`. The target is a name or a field path,
written the same way on both sides of the assignment. A body that tests the
element, appends two values, or writes anywhere else has no spread form.

Only the value form is checked; a loop that appends `src[i]` is left alone.

A loop that widens each element is left alone, since `append(dst, src...)`
takes a slice of the element type where `append(dst, v)` accepts any `v`
assignable to it. A loop over a map, an array, or a string is left alone, since
`append` spreads only a slice. A target that is a type parameter whose type set
holds one slice type counts as a slice, and the element type comes from the
type set.

A target that is a call is left alone, since the loop calls it twice per
element and the replacement calls it twice in total. A target that is a
dereference or an index is left alone, since the alias check follows a name and
these give it none; an index may also be the loop's key, where each pass
appends to a different element.

A loop whose range expression names the target is left alone, since a bound can
reach past the target's length, as in `range dst[:cap(dst)]`.

The source and the target can be two views of one array. Where the source
reaches past the target's length, the loop reads elements that earlier passes
overwrote. The spec calls the spread append's result "independent of whether
the memory referenced by the arguments overlaps", so the loop and the call
write different values.

A loop over the target itself reads only the elements before the target's
length, which the appends do not write. That loop cannot read what it writes,
so it is the one that carries a fix. The fix is withheld when a comment sits
inside the loop, since the edit would delete it.

Report only: the target and the source are usually parameters, so a caller can
pass two views of one array.

The rule is inspired by go-critic's `rangeAppendAll`, which reports the
opposite loop.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/rangeappendall/cmd/rangeappendall
go tool rangeappendall ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -rangeappendall ./...
go tool lesiw.io/tools/cmd/fix -rangeappendall ./...
```
