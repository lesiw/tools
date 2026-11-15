# lesiw.io/tools/analyzers/redundantsprint

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/redundantsprint.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/redundantsprint)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `fmt.Sprint` whose operand is already a
string.

```go
func f(s string) string {
    return fmt.Sprint(s) // redundant wrapper; the operand is already a string
}
```

A `fmt.Sprint` of a string and a conversion of a string to `string` both leave
the operand unchanged. Sprint formats an operand with `%v`, and `%v` writes a
string as itself, so a call on a single string returns it unchanged. A
conversion returns the same value.

One finding covers a chain of wrappers, and the fix leaves the innermost
operand: both `string(fmt.Sprint(s))` and `fmt.Sprint(fmt.Sprint(s))` become
`s`. The chain must hold a `fmt.Sprint` of a string: a conversion of a string to
`string` on its own is left alone.

The operand's static type must be `string`: an untyped string constant and an
alias of `string` both qualify. A named string type does not: `Sprint` may call
its `String` method, and its type is not `string`. So `word(fmt.Sprint(s))`
becomes `word(s)` for a named type `word`, and `string(fmt.Sprint(w))` stays for
a `w` of that type. A type parameter may be instantiated with a named string
type, and an interface value may hold one, so both are left alone.

Only a call with exactly one operand is judged: `fmt.Sprint()` takes none, and
`fmt.Sprint(a, b)` of two strings is their concatenation, which the rule does
not build. Only a package-qualified `fmt.Sprint` call is checked, so a call
through a function value or a dot import is left alone. `fmt.Sprintf` is left
alone: whether it leaves a value unchanged depends on the format string, which
this rule does not read.

An expression statement, `go`, and `defer` need a function call, so they are
reported only when the operand is one: `go fmt.Sprint(fn())` becomes `go fn()`,
and `go fmt.Sprint(s)` stays. A conversion or a builtin in these positions would
not compile, so `go fmt.Sprint(string(b))` stays too. `unusedresult` reports the
discarded result of an expression statement.

The rule is adapted from go-critic's `redundantSprint`, which also rewrites a
`fmt.Sprint` of a `fmt.Stringer` to a call of its `String` method.

Fixable: the wrappers are dropped and the operand is left where it was.
Parentheses written around the wrapper or around the operand go with it. A pair
of parentheses is added when the wrapper's value is indexed or sliced and the
operand is a binary, unary, or pointer expression, so `fmt.Sprint(a + b)[0]`
becomes `(a + b)[0]`. A comment inside the dropped text withholds the fix,
because the comment belongs to the author.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/redundantsprint/cmd/redundantsprint
go tool redundantsprint ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -redundantsprint ./...
go tool lesiw.io/tools/cmd/fix -redundantsprint ./...
```
