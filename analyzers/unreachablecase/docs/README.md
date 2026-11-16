# lesiw.io/tools/analyzers/unreachablecase

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unreachablecase.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unreachablecase)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a type in a type switch case that an
earlier interface case matches.

```go
func f(x any) {
    switch x.(type) {
    case io.Reader:
    case *bytes.Buffer: // unreachable type in case: io.Reader already matches every *bytes.Buffer; an earlier case is tested first
    }
}
```

A type switch tests its cases in source order. A case with an interface type
matches every value whose type implements it, so the case for a later type that
implements that interface never runs. The same holds for a later interface that
implements the earlier one.

Left alone:

- A nil case, which matches a nil interface; an interface case does not match
  nil.
- A default clause, which lists no type.
- A type parameter in an earlier case: not treated as an interface case, even
  when its constraint has only one type. A later type is not reported because
  of an earlier type parameter.
- A type parameter in a later case: checked against the earlier interface,
  using the constraint.
- The other types in one case. Types in a case are alternatives, so a case can
  still match through a type that does not implement the earlier interface. Only
  a type that an earlier interface already matches is reported.
- An earlier concrete type, which matches only itself; naming the same type
  twice does not compile.

No fix is offered. Moving the case before the earlier one makes it reachable,
which changes behavior, and where it belongs is a choice outside the rule.
Removing a type that is alone in its clause would move the body. Several
reported types can appear in one clause, and a comment can follow any type in a
multi-type clause, so one fix would overlap another or delete text.

The rule is adapted from staticcheck's SA4020.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unreachablecase/cmd/unreachablecase
go tool unreachablecase ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unreachablecase ./...
```
