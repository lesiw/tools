# lesiw.io/tools/analyzers/builtinshadow

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/builtinshadow.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/builtinshadow)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a declaration that shadows a predeclared
identifier.

```go
var len = 3 // declaration shadows the predeclared identifier "len"

func new(x int) int { // declaration shadows the predeclared identifier "new"
    return x
}
```

The Go spec declares the predeclared identifiers in the universe block
(https://go.dev/ref/spec#Predeclared_identifiers) and lets a declaration in an
inner scope shadow one in an outer scope
(https://go.dev/ref/spec#Declarations_and_scope). A package-level declaration
shadows the predeclared identifier for the package block. A local declaration
shadows it for the rest of the declaration's scope. The name denotes the new
declaration there, so later references to the name resolve to the declaration
or fail to compile.

The rule checks a package-level var, const, type, or func; a local var, const,
or type; a short variable declaration; a range or type switch variable; a
parameter, result, receiver, or type parameter; and an import name. A
declaration is reported for reusing a predeclared name, even when an outer
declaration already uses it.

Not reported:

- a parameter or result name in a function type, and a parameter, result, or
  receiver name in a declaration with no body, because no code can refer to
  those names;
- a struct field, an interface method, or a method name, which are selected
  through a type and do not shadow a lexical name;
- the blank identifier, which is not a name;
- a label, whose name is in a separate namespace;
- a predeclared name that the declaration's language version does not have:
  any and comparable before Go 1.18, and clear, max, and min before Go 1.21.

The version thresholds come from the Go 1.18 and Go 1.21 release notes
(https://go.dev/doc/go1.18, https://go.dev/doc/go1.21). A file that records no
version is treated as modern, so every predeclared name is available there. A
package-level declaration is visible to every file in the package, so it is
reported when any file's version predeclares the name.

Report only: a rename would have to update every use in the declaration's
scope, and the declaration may be intentional.

The rule is adapted from revive's redefines-builtin-id
(https://github.com/mgechev/revive/blob/HEAD/RULES_DESCRIPTIONS.md#redefines-builtin-id).

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/builtinshadow/cmd/builtinshadow
go tool builtinshadow ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -builtinshadow ./...
```
