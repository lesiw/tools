# lesiw.io/tools/analyzers/assertnil

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/assertnil.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/assertnil)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a redundant nil check on the result of a type assertion.

```go
type sink interface{ Use() }

func f(x any) {
	if s, ok := x.(sink); ok && s != nil { // s is never nil when ok is true
		s.Use()
	}
}
```

The Go FAQ says an interface value is nil only when its type and value are
both unset, so an assertion to an interface type yields a result that is never
nil.

The rule leaves alone a result whose type is not an interface, since a pointer,
or a type parameter instantiated with one, can yield nil with `ok` true. It
leaves alone a condition in which another operand can write the result or the
guard, and one where the comparison comes before the guard with an operand
between them, since there the check is what keeps that operand from running. It
reads the `if` init, so a `for` or `switch` init is left alone, and so is an
assertion made earlier and tested in the `if`. A comparison with `== nil` or a
negated guard leaves a body that never runs, and this rule does not report dead
branches.

The idea comes from staticcheck's S1020, Omit redundant nil check in type
assertion: S1020 tests the value the assertion is applied to, and this tests the
result.

Fixable: the nil comparison is removed, and the result is bound to `_` where
nothing else reads it. The fix is withheld where the deleted text holds a
comment, and where nothing but the check reads the value and something still
assigns it.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/assertnil/cmd/assertnil
go tool assertnil ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -assertnil ./...
go tool lesiw.io/tools/cmd/fix -assertnil ./...
```
