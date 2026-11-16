# lesiw.io/tools/analyzers/rangelast

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/rangelast.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/rangelast)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a range loop that keeps only the last
element of a slice or array.

```go
func last(xs [3]int) (v int) {
	for _, x := range xs { // loop only keeps the last element
		v = x
	}
	return v
}
```

The specification's "For statements with range clause" section
(https://go.dev/ref/spec#For_range) says that a range over an array or slice
produces the elements in increasing index order, from `0` to `len(x)-1`. The
loop assigns every element in turn, and the last assignment sets the value, so
`x[len(x)-1]` reads the same element in one expression.

The loop must declare its value with `:=`; a loop with `=` also writes the
value variable after the loop, so it is left alone. The key must be blank; a
named key is also assigned on every pass, so the loop keeps more than the last
element. The body must be exactly one assignment of the range value.

The target is a variable or a field path whose bases are struct values. An
element, a dereference, and a field reached through a pointer can write a
location the range reads later, so they are left alone. A target that refers
to the range value is left alone too. A blank target is left alone.

The range expression must be an array or a slice. A pointer to an array counts
as an array. A type parameter counts when every type in its constraint is a
slice or array; the type set of a rangeable array or slice type parameter has
one underlying type, so the surviving array terms share a length. A string is
left alone because a range yields
runes where an index yields a byte. A map has no index order and a channel has
no index.

The specification's Index expressions section
(https://go.dev/ref/spec#Index_expressions) says an out-of-range index panics,
that indexing a string yields a byte, and that a[x] is shorthand for (*a)[x]
when a is a pointer to an array. The Length and capacity section
(https://go.dev/ref/spec#Length_and_capacity) defines len.

The rule leaves alone an empty value whose emptiness it can determine: an array
of length zero, a pointer to one, a slice literal with no elements, or a type
parameter whose every type is empty. A slice whose length the rule does not
prove is reported without a fix, since an empty value leaves the target as it
was and the replacement's index panics.

Fixable: the loop becomes `target = x[len(x)-1]`. The fix is attached when the
value is known to be non-empty: an array or a pointer to one of positive
length, a slice literal with an element, or a type parameter whose every type
is non-empty. The
replacement contains the range expression twice, so a slice literal is built
twice; the literal has no side effect, so the two values agree.

The fix is withheld when evaluating the range expression twice can give a
different value, since the loop evaluates it once and the replacement evaluates
it twice. It is withheld when a comment sits inside the loop, since the
replacement would remove the comment, when the replacement would span lines, as
a raw string in a literal does, and when `len` refers to a declaration instead
of the builtin.

The rule is original to this suite.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/rangelast/cmd/rangelast
go tool rangelast ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -rangelast ./...
go tool lesiw.io/tools/cmd/fix -rangelast ./...
```
