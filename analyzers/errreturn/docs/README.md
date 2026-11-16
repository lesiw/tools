# lesiw.io/tools/analyzers/errreturn

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errreturn.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errreturn)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if that returns the value it tests
against nil, followed by a return of nil.

```go
func f(err error) error {
    if err != nil { // redundant if; write return err
        return err
    }
    return nil
}
```

The branch returns the value when it is not nil, and the trailing return
yields nil otherwise, so `return err` covers both.

When the if declares the value in its init statement, the replacement returns
the declaration's expression, since the variable does not exist outside the if:

```go
func g() error {
    if err := load(); err != nil { // redundant if; write return load()
        return err
    }
    return nil
}
```

The condition must compare a name with nil using `!=`, and the body must be
one return of that same name. The value must have the function's result type.
A concrete value returned as an interface is left alone, because a nil value
of the concrete type becomes a non-nil interface, where the trailing return
yields nil. The trailing return must be the next statement and must return
nil. A different value, a comparison that uses `==`, an extra statement, an
else branch, and a result that is not nil are left alone. An if with an init
is reported only when the init is a short declaration of one name from one
expression, and that name must be the one the condition tests. The assignment
form is left alone because the write may be observable outside the if, and
more than one name has no single value to return.

The fix replaces the if and the trailing return with the one return, written
on one line. An init expression with a one-line form is collapsed onto that
line. It is withheld, and the pair is still reported, when a comment inside
the pair would be deleted, when the resulting line would be over the width
limit that `linelen` configures, and when the init expression has no one-line
form, as a raw string does.

The rule is adapted from revive's `if-return`, which reports the form with an
init declaration. The rule also reports the form without one.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errreturn/cmd/errreturn
go tool errreturn ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errreturn ./...
go tool lesiw.io/tools/cmd/fix -errreturn ./...
```
