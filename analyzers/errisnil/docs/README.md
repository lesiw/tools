# lesiw.io/tools/analyzers/errisnil

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errisnil.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errisnil)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an error compared with nil twice in one
boolean expression.

```go
func f(err error) bool {
    return err != nil && err != nil // error compared with nil twice in the same expression
}
```

The spec's Logical operators section
(https://go.dev/ref/spec#Logical_operators) says "The left operand is
evaluated, and then the right if the condition requires it." The second test
has the same result as the first, so it cannot change the result.

The rule reports a repeated test joined by `&&` or `||`, with the same test
both times: `err != nil && err != nil`, `err == nil || err == nil`,
`err != nil || err != nil`, and `err == nil && err == nil`. The comparison has
to test the predeclared nil
(https://go.dev/ref/spec#Predeclared_identifiers) and a value whose type is
assignable to `error` (https://go.dev/ref/spec#Assignability). A type parameter
is left alone unless its constraint implements error
(https://go.dev/ref/spec#Implementing_an_interface). A mixed pair,
`err != nil && err == nil`, tests two different outcomes and is left alone. A
variable named nil is not the predeclared nil.

Only tests joined by the same `&&` or `||` are compared. A repeat across two
operators, as in `err != nil && (ok || err != nil)`, is left alone.

A test whose error expression contains a receive, an address of a composite
literal, or a call other than a conversion or one of these builtins is left
alone; such an expression may give a different value. A repeat with such
an expression between the two tests is left alone too. A conversion or one of
`cap`, `complex`, `imag`, `len`, `max`, `min`, and `real` does not stop the
report; an effectful call inside it still does. A type assertion does not stop
the report. The spec defines a conversion
(https://go.dev/ref/spec#Conversions), and the builtin documentation lists each
of those functions (https://pkg.go.dev/builtin). The length of a channel can
change between two reads
(https://go.dev/ref/spec#Length_and_capacity), so an error expression that
reads one, or a type parameter whose type set may hold one, is left alone too.
The capacity of a channel does not change, so a `cap` read is not exempt.

A repeat inside a parenthesized chain is reported. The fix is withheld when the
repeat is the first operand of that chain, as in
`err != nil && (err != nil && ok)`, because the edit would cross the
parentheses.

The fix drops the repeated comparison and the operator before it, so
`err != nil && ok && err != nil` becomes `err != nil && ok`. A report with a
comment overlapping the text the fix would delete has no fix.

The suite's `bools` pass reports repeated operands in a boolean chain too, so a
suite run can report both diagnostics. This rule adds the fix and also reports
a reversed comparison such as `err != nil && nil != err`.

The rule is adapted from staticcheck's SA4000 family.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errisnil/cmd/errisnil
go tool errisnil ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errisnil ./...
go tool lesiw.io/tools/cmd/fix -errisnil ./...
```
