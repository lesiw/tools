# lesiw.io/tools/analyzers/errorsastypeshadow

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errorsastypeshadow.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errorsastypeshadow)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an `errors.AsType` call that tests a zero
value from an earlier call in the same if/else chain.

A call that fails returns the zero value of its type parameter, and an else
branch runs only when the call above it failed. A later branch that passes the
variable the earlier call declared passes its zero value:

```go
if err, ok := errors.AsType[*alphaError](err); ok {
    use(err)
} else if err, ok := errors.AsType[*betaError](err); ok { // err is the zero value of *alphaError from the earlier AsType call
    use(err)
}
```

The second call passes the variable the call above declared, so it tests a nil
`*alphaError` whatever the original error was. Its answer says nothing about the
original error.

The chain runs from the call that declares the variable to the call that passes
it, and both ends are the same shape: a short variable declaration in an if
statement's init, whose condition is the second name declared, as in
`if err, ok := errors.AsType[T](x); ok`. A statement whose condition tests
anything else, as `ok && enabled` does, is not a link in the chain.

A use between the two calls suppresses the report, except in three positions:
an argument, the right side of an assignment, and an operand of a binary
expression. The analyzer climbs from the use through a selector, an index
expression, a type assertion and parentheses, and reads the position it
reaches. Every other position suppresses the report, including a method call on
the variable, a dereference, a composite literal, and a use as the subscript of
an index expression.

Only one branch of a chain runs, so an intervening `else if` is examined only at
its init statement and its condition. When the call sits inside a branch body,
that body is examined whole, so a write after the call suppresses the report
too.

The rule compares variables, not names. A chain that passes the original error
to every call is left alone, and so is a call made in the ok branch of the call
that declared the variable.

The `errors` and `errors_test` packages are exempt, as in gopls: those packages
know their own API, and a test writes chains that are wrong on purpose.

Report only: the repair renames a variable, and only the author can choose the
name.

Vendored from gopls' `errorsastypeshadow`
(`gopls/internal/analysis/errorsastypeshadow`, `golang.org/x/tools/gopls`
v0.24.0), which is internal and cannot be imported. The rule is upstream's; the
code is adapted to this repository's helpers.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errorsastypeshadow/cmd/errorsastypeshadow
go tool errorsastypeshadow ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errorsastypeshadow ./...
```
