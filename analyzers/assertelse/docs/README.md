# lesiw.io/tools/analyzers/assertelse

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/assertelse.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/assertelse)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a type assertion whose value is read in the
branch that runs when the assertion fails.

```go
func report(x any) {
    if v, ok := x.(int); ok {
        print(v)
    } else {
        print(v) // v is the zero value in the branch where the type assertion fails
    }
}
```

A two-value type assertion yields a value and a success result. The Go spec says
the value is the zero value of the asserted type when the assertion fails
(https://go.dev/ref/spec#Type_assertions). An if statement is an implicit block
(https://go.dev/ref/spec#Blocks), and a variable declared in its init is in
scope to the end of that block
(https://go.dev/ref/spec#Declarations_and_scope), so the branch entered on
failure can read that zero value.

The rule reports an if statement whose init makes a two-value type assertion and
whose condition tests the success result directly, as `ok` or `!ok`, when the
branch entered on failure reads the value. A read of the value, a field or an
element counts, and so does a read in an else-if condition or body or a read
that passes the value to another call. The init may declare the value with `:=`
or assign an existing local variable with `=`. One report is made for each if
statement. A read used only for its type, as in a `%T` format, is reported
because the value is read in the failing branch.

Not reported:

- a condition that combines the success result with another test; only a
  condition that is the success result or its negation is checked;
- a read that follows an assignment to the value in the failing branch, or to a
  field of a struct or an element of an array held in a local; an assignment to
  any field or element before the read suppresses the report;
- a value whose address is taken or that a function literal captures anywhere
  in the function, since a later read can go through the pointer or closure;
- a bare identifier assigned to a blank identifier, as in `_ = v`;
- a value named `_`, and an if statement with no branch for the failure;
- a one-value type assertion, which panics on failure;
- a read whose value is merged with an assignment on another path;
- an argument to `unsafe.Sizeof`, `unsafe.Alignof`, or `unsafe.Offsetof`,
  since those arguments are not evaluated;
- a type switch, whose default clause binds the value being switched on.

Report only: reading the asserted expression and handling the failure are both
plausible repairs.

The rule is adapted from staticcheck's SA9008
(https://staticcheck.dev/docs/checks/#SA9008).

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/assertelse/cmd/assertelse
go tool assertelse ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -assertelse ./...
```
