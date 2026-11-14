# lesiw.io/tools/analyzers/unnecessaryblock

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unnecessaryblock.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unnecessaryblock)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a redundant block statement.

```go
func f(n int) {
	{ // redundant block statement
		println(n)
	}
	switch n {
	case 1:
		{ // redundant block statement
			x := 1
			println(x)
		}
	}
}
```

The declarations and scope section of the spec says the scope of a name
declared inside a function "ends at the end of the innermost containing block",
so a block that declares nothing scopes nothing.

A block that only groups statements under a comment is reported too, and the
comment is kept.

A block that declares a name in a statement list keeps its braces: the rule
does not judge where a name's scope should end. A clause is the exception. The
spec's Blocks section says each clause acts as an implicit block, so a block
that is the whole of a case or comm clause is reported even when it declares a
name, since nothing outside the block can observe the scope the braces add. The
braces stay when the clause already declares a name the block declares: the
moved statement would redeclare that name, or assign to it.

A block keeps its braces when a `goto` in the block around it jumps past it. Go
rejects a jump over a variable declaration, blank or not, and a generator may
put one there, as the Go runtime's malloc stubs do.

The rule is adapted from go-critic's unnecessaryBlock.

Fixable: the braces are removed, and an empty block leaves no blank line
behind.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unnecessaryblock/cmd/unnecessaryblock
go tool unnecessaryblock ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unnecessaryblock ./...
go tool lesiw.io/tools/cmd/fix -unnecessaryblock ./...
```
