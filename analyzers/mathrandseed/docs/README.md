# lesiw.io/tools/analyzers/mathrandseed

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/mathrandseed.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/mathrandseed)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to `math/rand.Seed`.

```go
func seeded() {
    rand.Seed(1) // math/rand.Seed is deprecated; use New(NewSource(seed)) for a fixed sequence
}
```

The `math/rand` package documentation says the global generator is seeded
randomly at program startup when `Seed` is not called, and marks `Seed`
deprecated as of Go 1.20. From Go 1.24 `Seed` is a no-op by default;
`GODEBUG=randseednop=0` restores its effect. For a fixed sequence, call
`New(NewSource(seed))`.

A call to the package-level function is reported through any import name,
including a dot import. A call inside package `math/rand`, or in its external
test package, is left alone, because `Seed` is declared there and tested by
both. A call to the `Seed` method of a `*rand.Rand`, a field named `Seed`, a
function value, or a `Seed` declared in another package is left alone.

Fixable: the statement that contains the call is deleted. The line that
contains the statement is deleted when the statement is alone on it; otherwise
only the statement is removed. A following semicolon is removed too unless the
statement is a for loop's init, where the semicolon separates the init from the
condition. A comment or other code on the line stays. A fix can leave an import
unused; goimports removes it.

The fix is withheld when the statement is labeled, which would leave the label
without a statement; when the statement contains a comment; when the call comes
from a dot import in the call or its argument; when the argument is not a
constant expression, an identifier, or a unary form of one that cannot block or
panic; or when a local variable has no read outside Seed calls. A dot import
has no fix because deleting its last use would leave the import unused, and
goimports does not remove a dot import.

The fix is offered only in package main of the main module, where a
`//go:debug` line is visible. It is withheld when the go directive in the main
module's go.mod is below Go 1.24, when the module's go.mod or a loaded
`//go:debug` line has a godebug setting that restores Seed's effect, when a
go.work file is in effect, or when the driver supplies no module Go version. A
`GODEBUG` setting in the environment is outside the analysis. The call is still
reported. A `//go:debug` line in another package, or in a test file the driver
does not load, is not seen.

A local variable whose only reads are inside `Seed` calls is left without a
fix, so a run of `Seed` calls that share one variable is reported without a
fix.

The rule is adapted from staticcheck's SA1019, which reports a use of a
deprecated identifier only when the target version has the deprecation. This
rule reports the call at every version.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/mathrandseed/cmd/mathrandseed
go tool mathrandseed ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -mathrandseed ./...
go tool lesiw.io/tools/cmd/fix -mathrandseed ./...
```
