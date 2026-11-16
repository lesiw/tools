# lesiw.io/tools/analyzers/sortnosort

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/sortnosort.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/sortnosort)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an assignment that converts a slice to a
`sort` slice type and assigns it back.

```go
func f(names []string) {
    names = sort.StringSlice(names) // sort.StringSlice is a type, so the assignment sorts nothing; write sort.Strings(names)
}
```

The `sort` package declares `StringSlice`, `IntSlice`, and `Float64Slice` as
types. The [`StringSlice`](https://pkg.go.dev/sort#StringSlice) doc says it
"attaches the methods of Interface to []string, sorting in increasing order".
The conversion yields a value of that type, and the assignment stores it back
in the target. Nothing calls the methods, so `names = sort.StringSlice(names)`
sorts nothing.

The assignment must use `=`, with one target expression on the left and one
expression on the right. The right side must be a conversion to
`sort.StringSlice`, `sort.IntSlice`, or `sort.Float64Slice`, and its argument
must be the same expression as the target. Parentheses around either side do
not matter. The rule reports a target whose type is an unnamed slice, or an
alias of one. A conversion written through an alias of one of the three sort
types is reported, wherever the alias is declared. The target may be an
identifier, a selector, an index expression, or a dereference. A slice
expression with no bounds, such as `names[:]`, reads the same value as the
target and is reported. The analyzer compares identifiers, selectors, indexes,
literals, and dereferences, so any other more complex expression on either
side is left alone. An index is compared through its key, so an index whose
key is a binary expression, as in `rows[i+1]`, is left alone.

These assignments are left alone:

- a target whose type is already `sort.StringSlice`, `sort.IntSlice`, or
  `sort.Float64Slice`, where the conversion repeats the target's type and
  keeps the methods;
- a conversion of another expression, as in `names = sort.StringSlice(spare)`;
- a conversion whose argument is a slice expression with a bound, as in
  `names = sort.StringSlice(names[:0])`, where the argument is not the target
  expression; with a bound such as `:0`, the assignment can also change the
  slice header;
- a conversion to a distinct type named `StringSlice`, `IntSlice`, or
  `Float64Slice` declared in another package or locally.

`sort.Sort(sort.StringSlice(names))` and `sort.StringSlice(names).Sort()` are
not assignments, and both call the methods, so neither is reported. An
assignment with more than one target or more than one expression on either side
is left alone.

The fix replaces the whole assignment with a call to the matching function,
qualified by the name the conversion uses for `sort` or another name the file
imports `sort` under, as in `sort.Strings(names)` or
`stdsort.Strings(names)`. When the conversion does not name `sort`, a file that
imports `sort` with a dot uses the bare function name if that name reaches the
imported function. When the file lacks a usable import of `sort`, the fix adds
one, so a conversion through an alias from another package becomes
`sort.Strings(names)`.

The call sorts the slice, which the assignment did not do, so the suggested fix
message says that the edit changes behavior.

The fix is withheld when:

- a comment overlaps the assignment, since the replacement would delete it;
- a local declaration shadows the bare function name where the rewrite would
  use it, and the file has no usable named import of `sort` or the dot import
  has no other use;
- a name the rewrite writes reaches another object at the assignment;
- the only import of `sort` in the file is blank;
- the target is not addressable, as in an index into a map, where the
  assignment can insert a missing key and the rewrite only reads it;
- the fixes for the file's conversions through a dot import of another
  package would leave that import unused, so none of those fixes is offered;
- the package is `sort` or a package that `sort` imports, directly or
  transitively, where the new import would make a cycle.

The rule is adapted from staticcheck's `SA4029`, which reports the same three
conversions and replaces the assignment with a call to `sort.Strings`,
`sort.Ints`, or `sort.Float64s`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/sortnosort/cmd/sortnosort
go tool sortnosort ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -sortnosort ./...
go tool lesiw.io/tools/cmd/fix -sortnosort ./...
```
