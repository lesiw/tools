# lesiw.io/tools/analyzers/unusedparams

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unusedparams.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unusedparams)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a parameter a declared function never
uses.

```go
func unused(a int, b string) string { // parameter a is never used
    return b
}
```

A parameter that is never used is either a leftover or a signature the author
meant to keep. Renaming it to the blank identifier says which, and costs
nothing.

The rule leaves these alone:

- an exported function or method, which is documented by its parameter names;
- an unexported method whose name matches one an interface in the package
  declares. An unexported name cannot be satisfied from another package, so an
  interface declared elsewhere constrains nothing;
- any function used as a value, which may have to match a func type;
- a function whose body is empty or absent, or one that only panics, which is a
  placeholder.

Vendored from gopls' `unusedparams`
(`gopls/internal/analysis/unusedparams`, `golang.org/x/tools/gopls` v0.24.0),
which is internal and cannot be imported. The rule is upstream's; the code is
adapted to this repository's helpers.

Upstream also reports a func literal that is bound to a variable the package
never uses as a value. A literal is not a declaration, so this rule, which
reports declared functions, leaves every literal as written. Upstream's second
fix removes the parameter and the arguments passed for it at every call site,
and that takes a gopls command, so this rule offers the rename alone. Upstream
underlines the whole parameter field when it declares one name, where this rule
underlines the name.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unusedparams/cmd/unusedparams
go tool unusedparams ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unusedparams ./...
go tool lesiw.io/tools/cmd/fix -unusedparams ./...
```
