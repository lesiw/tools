# lesiw.io/tools/analyzers/flagname

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/flagname.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/flagname)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a flag name the flag package rejects.

```go
var level = flag.Int("-level", 0, "") // flag name "-level" begins with a hyphen
```

`FlagSet.Var` has panicked on such a name since Go 1.17. Before that, the
declaration registered a flag the usage message offered and no command line
could set: the parser takes one or two hyphens and then rejects a name that
still begins with one, and it ends the name at the first equal sign.

## Checks

The rule reports a name that begins with a hyphen or contains an equal sign. A
name with both faults is reported once, for the hyphen, which the package
checks first.

The rule reads every `flag` function that declares a flag, including a method
on a `*flag.FlagSet` and a method expression of one. `Set` and `Lookup` take a
name and declare nothing, so they are not read. A dot-imported `flag` is read; a
variable holding one of those functions is not.

The rule leaves a name that is not a constant alone, since it cannot see what
the name holds, and an empty name and a name containing a space, since the
package rejects neither.

## Fixes

A leading run of hyphens is dropped: `"-alpha"` becomes `"alpha"`. An equal
sign ends the name, and everything after it is dropped: `"addr=host"` becomes
`"addr"`. A name that still holds a space gets no fix, since the rule has
nothing to say about the space.

The old name has no readers to break: a declaration the rule reports panics
before it registers, so no running program reads the name.

The fix writes the name as a string literal, so a name that is not one gets no
fix. It is also withheld when nothing is left of the name, as with `"-"`, and
when another declaration in the package would take the same name, since
registering a name twice on one set panics. The count sees only the flag names
this package writes as constants: a name another package registers on
`flag.CommandLine` is invisible, and since the analyzer cannot tell which set a
call registers on, a declaration on a different set can block the fix.

The rule is adapted from go-critic's `flagName`, which reads the constructors
and their `Var` forms and reports an empty name and a name containing a space
as well. This rule reads `Var`, `TextVar`, `Func`, `BoolFunc`, and the
`FlagSet` methods too, and suggests a fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/flagname/cmd/flagname
go tool flagname ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -flagname ./...
go tool lesiw.io/tools/cmd/fix -flagname ./...
```
