# lesiw.io/tools/analyzers/appendunused

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/appendunused.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/appendunused)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an append result assigned to the blank
identifier, as `_ = append(xs, v)` or as one target of a multi-value
assignment or short declaration.

```go
func dropped(xs []int, v int) []int {
    _ = append(xs, v) // append to xs discards its result, so xs keeps its length
    return xs
}
```

The "Appending to and copying slices" section of the spec says that append
returns the updated slice
(https://go.dev/ref/spec#Appending_and_copying_slices). A slice header is
passed by value, so a call does not change the slice variable passed to it:
that variable keeps its old length, and the returned header has the new length.
Writing the result to the blank identifier discards that header.

The "Expression statements" section does not permit append in statement
context (https://go.dev/ref/spec#Expression_statements), so the form that
discards the result is `_ = append(xs, v)`.

The append may still write into spare capacity that the slice already has, and
those writes can be visible through another slice that shares the backing
array. Those writes do not affect the report; the rule reports the assignment
that drops the header with the new length.

A call that spreads a second slice, as `_ = append(xs, ys...)`, is reported the
same way: the result has the combined length, and the blank identifier discards
it. A reader who meant to grow the slice has to assign the result back to it.
The blank may also be one target of a multi-value assignment or short
declaration, as in `_, err = append(xs, v), f()`; the analyzer checks every
target. The first argument may be a parameter, a local variable, a package
variable, a value of a named slice type, or a type parameter.

Silent, deliberately:

- an append with no values after the slice, which this analyzer does not
  report; the x/tools `appends` pass reports it in the suite;
- a declaration of the blank identifier, as `var _ = append(xs, v)`; the rule
  visits assignments, so this declaration is outside its scope;
- a call whose first argument is not an identifier, as
  `_ = append([]int{v}, v)`, `_ = append([]byte(nil), b...)`,
  `_ = append(make([]int, 0, 1), v)`, `_ = append(xs[:1], v)`,
  `_ = append(h.items, v)`, or `_ = append(grown(), v)`. The rule matches the
  identifier form only. To make a deliberate spare-capacity write explicit,
  write the first argument as a slice expression, as in
  `_ = append(buf[:0], b...)`; that form is outside the rule. Fresh slices and
  call results are outside the match as well;
- a call to another declaration named append, as in `append := func(...)` or a
  method named append; only the predeclared builtin is matched.

`appendassign` reports a result assigned to another slice when the function
still reads the first argument. This rule reports the blank target that
`appendassign` does not.

An assignment or declaration that saves the result, as `xs = append(xs, v)`, is
outside this rule even when nothing reads `xs` afterwards. staticcheck's SA4010
reports that form when the saved result is never used and the backing array is
not aliased.

Report only: assigning the result to the slice grows the slice, and removing
the append can drop evaluation of its arguments as well as the write into
spare capacity. Neither edit is safe without knowing the intent.

The rule is adapted from staticcheck's SA4010, which reports an append result
that is never used. SA4010 also checks that the backing array is not aliased;
this rule reports the blank assignment without that check.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/appendunused/cmd/appendunused
go tool appendunused ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -appendunused ./...
```
