# lesiw.io/tools/analyzers/sortstrings

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/sortstrings.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/sortstrings)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports `sort.Strings`, `sort.Ints`,
`sort.Float64s`, or a `sort.Slice` whose less function compares the two
elements at the indexes it was given, where the element type is ordered.

```go
func f(names []string) {
	sort.Strings(names) // sort.Strings can be replaced by slices.Sort
}
```

The [`slices.Sort`](https://pkg.go.dev/slices#Sort) doc says it "sorts a slice
of any ordered type in ascending order". The
[`sort.Float64s`](https://pkg.go.dev/sort#Float64s) doc says it calls
`slices.Sort` as of Go 1.22 and places NaNs before other values; the `Strings`
and `Ints` docs say they call `slices.Sort` as of the same release. The
[comparison operators](https://go.dev/ref/spec#Comparison_operators) section
of the spec defines `<` for ordered operands. A `sort.Slice` call whose less
function is `xs[i] < xs[j]` produces the order `slices.Sort` produces, up to
elements that compare equal.

The call must be `sort.Strings`, `sort.Ints`, `sort.Float64s`, or
`sort.Slice`. `sort.Sort(sort.StringSlice(xs))` is outside this rule: S1032
stops at the element-typed call, and this rule starts there.
`slices.SortStableFunc` takes a comparison in place of `sort.SliceStable`, and
`slices.IsSorted` returns a boolean in place of `sort.SliceIsSorted`.

For the rule to report a `sort.Slice` call, the argument's type set must hold
slices with one element type. `sort.Slice` accepts any value and panics on
anything else, so a map, array, or string argument is left alone and reported
by x/tools' `sortslice`. A type parameter whose constraint holds such slices is
accepted.

A `sort.Slice` less function must be a func literal whose only statement
returns a `<` comparison of the two elements at the indexes it was given. The
first index reads the left operand and the second reads the right. A `>` or
`<=` comparison is left alone, and so is `xs[j] < xs[i]`. A body with another
statement, another comparison, or another operand is left alone.

The slice argument and the value indexed in the comparison must be the same
expression, compared by the objects the names reach. The argument may be an
identifier, a field, an index, or a dereference. A call, a receive, a type
assertion, a slice expression, a conversion, and a composite literal are left
alone. A `sort.Slice` written as one multi-valued call is left alone.

A `sort.Slice` over an element type that can hold a NaN is left alone because
`slices.Sort` places NaNs before the other values where the supplied `<` does
not. `sort.Float64s` is reported because the `sort` package already places
NaNs first.

The `<` operator type-checks for an ordered type. A named type whose underlying
type is ordered is accepted, and so is a type parameter whose type set holds
only such types. A struct, a pointer, a map, and an interface are left alone.

`sort.Strings(nil)`, `sort.Ints(nil)`, and `sort.Float64s(nil)` are left alone,
because `slices.Sort` has no element type to infer.

The analyzer resolves the callee, so it reports a call through a dot import of
`sort`.
The fix is withheld when the replacement would leave that dot import unused,
because goimports does not remove a dot import.

A file below Go 1.21 is left alone, because `slices.Sort` does not exist there.

The rule is adapted from staticcheck's `S1032`, which reports
`sort.Sort(sort.StringSlice(x))`, `sort.Sort(sort.IntSlice(x))`, and
`sort.Sort(sort.Float64Slice(x))` in favor of the element-typed functions, and
from x/tools' modernize `slicessort`, which reports the `sort.Slice` form. This
rule reports those element-typed functions, and the `sort.Slice` form with a
slice argument and an element type that cannot hold a NaN, in favor of
`slices.Sort`. The suite runs this rule in place of `slicessort`, whose fix can
delete a comment and move a NaN. `slicessort` also reported `sort.Slice` over a
NaN-capable element type and over arguments this rule leaves alone, so the
suite does not report those calls.

Fixable: the call becomes `slices.Sort(x)`, and the import is added where the
file lacks one. An import under an alias is used as written, as in
`sl.Sort(x)`.

The fix is withheld when:

- a comment sits inside the call;
- the replacement would leave a dot import of `sort` unused;
- the name used for `slices` at the call reaches another object;
- the file binds `slices` to the dot or blank name;
- the package is one that `slices` imports, where the new import would make a
  cycle.

A dot import of `slices` is left alone because a second name for `slices`
would duplicate the import. A blank import gives the fix no name to call.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/sortstrings/cmd/sortstrings
go tool sortstrings ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -sortstrings ./...
go tool lesiw.io/tools/cmd/fix -sortstrings ./...
```
