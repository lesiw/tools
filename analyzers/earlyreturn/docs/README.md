# lesiw.io/tools/analyzers/earlyreturn

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/earlyreturn.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/earlyreturn)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a return that chains conditions past the
width.

```go
func matches(s string) bool {
	return strings.Contains(s, "alpha-one") && strings.HasPrefix(s, "alpha") || // chains 4 conditions
		strings.Contains(s, "beta-two") && strings.HasPrefix(s, "beta")
}
```

A chain that mixes `&&` with `||` makes the reader work out the precedence as
well as the conditions, and a second line makes that worse. Early returns
decompose it:

```go
func matches(s string) bool {
	if strings.Contains(s, "alpha-one") && strings.HasPrefix(s, "alpha") {
		return true
	}
	return strings.Contains(s, "beta-two") && strings.HasPrefix(s, "beta")
}
```

The rule reports the shape and points at the remedy. It does not rewrite,
because decomposition changes control flow and the fix would need to name each
part.

A return whose value is not a chain of conditions is left alone however it
wraps. The fault is a return of a condition that cannot hold one line, where
one test has become several.

A chain that uses one operator kind is left alone however long it is. A
conjunction of three has no precedence to work out, and Go code is full of long
conjunctions that read straight down.

A header that wraps has the same problem, and [`linewrap`](../../linewrap)
reports it.

The width is `linelen`'s. With no width there is nothing to overflow, so the
rule stands down.

## Usage

    go tool lesiw.io/tools/cmd/vet -earlyreturn ./...
