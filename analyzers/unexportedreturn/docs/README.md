# lesiw.io/tools/analyzers/unexportedreturn

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unexportedreturn.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unexportedreturn)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an exported function or method whose result
is an unexported type declared in the same package.

```go
func New() option { // exported func New returns unexported type option
    return option{}
}
```

An identifier is accessible from another package only when it is exported, so a
caller cannot write the type the result is declared with.

A pointer to the type is reported too, and the diagnostic names `*option`. Every
result is reported, so a function with two unexported results gets two
diagnostics. The rule does not look inside `[]option`, `func() option`, or
`Box[option]`.

With `type Option = option`, `func New() Option` and `func New() *Option` both
go unreported, and a caller writes `a.Option`. An unexported alias is followed
to the type it names: `type secret = option` is reported as `secret`, and an
unexported alias for an exported type, such as `type helper = Config`, is left
alone.

A method is left alone when its receiver names a type the package does not
export, as in `func (hush) AliasPlain() option`,
`func (handle) Value() option`, and `func (*handle) Pointer() option`.
`func (helper) AliasRecv() option` and `func (Handle) Alias() option` are
reported, because a caller reaches the method through `Config` and `Handle`.

Not reported, and why:

- a result that is an interface, or a pointer to one;
- a method declared in an interface;
- an internal package, whose importers are in the same tree;
- `package main`, which cannot be imported;
- a test file, whose exported code only the package's own tests can call.

The rule is adapted from revive's `unexported-return`, which also reports an
unexported type behind a map, slice, array, or channel, and stops after the
first unexported result.

Report only: the repair needs an exported name for the type. Declare
`type Option = option` and use it as the result type, or export the type.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unexportedreturn/cmd/unexportedreturn
go tool unexportedreturn ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unexportedreturn ./...
```
