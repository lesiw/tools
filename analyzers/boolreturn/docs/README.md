# lesiw.io/tools/analyzers/boolreturn

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/boolreturn.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/boolreturn)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports two returns of opposite boolean constants under an if.

```go
func f(ok bool) bool {
	if ok { // two returns of opposite constants; write return ok
		return true
	}
	return false
}
```

The condition is the value the function returns. Testing it, then spelling the
answer in both branches, says the same thing twice.

With the branches reversed the answer is the condition negated. A comparison
takes its opposite operator, a `len`, `cap` or `copy` count against zero takes
`==`, and a float comparison takes a `!` around the whole expression, since NaN
makes both halves false:

```go
func g(n int) bool {
	if n == 0 { // two returns of opposite constants; write return n != 0
		return false
	} else {
		return true
	}
}
```

Some pairs cannot be rewritten, and are not reported:

- an if with an init statement, because a return has nowhere to put the init;
- a link in an `else if` chain, because an `else` takes a block or an if;
- a condition that is itself a conjunction or a disjunction, when the branch
  that returns false comes first, because a `!` around the whole expression
  reads worse than the if;
- a condition whose type the function does not return, because the return would
  not compile;
- a condition the printer cannot lay out on one line, as a raw string cannot,
  because the replacement is a single return.

One shape is reported without a fix: a pair whose return would run past the
width limit, because the wrap is the author's to place. Comments inside the pair
move above the return that replaces it.

The idea comes from staticcheck's S1008, which collapses the pair that ends a
block and leaves one below a guard alone.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/boolreturn/cmd/boolreturn
go tool boolreturn ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -boolreturn ./...
go tool lesiw.io/tools/cmd/fix -boolreturn ./...
```
