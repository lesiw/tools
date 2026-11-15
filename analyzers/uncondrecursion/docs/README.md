# lesiw.io/tools/analyzers/uncondrecursion

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/uncondrecursion.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/uncondrecursion)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a recursive call that runs whenever the
function runs.

```go
func f() {
	f() // unconditional recursive call
}
```

A function that calls itself on every entry recurses without end. The
goroutine's stack grows until it reaches a limit, and then the program crashes
with `fatal error: stack overflow`. `runtime/debug.SetMaxStack` is documented
as "useful mainly for limiting the damage done by goroutines that enter an
infinite recursion". A call in a go statement starts a goroutine on every
entry instead, so the function returns and the program starts goroutines
forever. A deferred call runs as the frame unwinds, so it recurses when it
returns.

The rule reports a call the function makes to itself in its own body, and only
when the call runs whenever the function runs:

- on every entry, or as the frame unwinds through a `defer`;
- in the body of a `for` that has no condition;
- in the callee or an argument of a go statement;
- in the expressions a statement evaluates before any branch is taken: the
  init, the condition, a switch's tag or type-switch operand, a range's operand,
  and a select's channel operands and sent values.

A call is left alone when:

- it sits inside the body of a branch, a `select` case, a `for` that has a
  condition, or a `range`, where it runs only when execution reaches it;
- it follows a `return` or a call that ends the goroutine, or an `if`, `switch`,
  `select`, `for`, or `range` that can leave the function, since the call is
  skipped on that path. A `panic`, an `os.Exit`, a `runtime.Goexit`, a
  `log.Fatal`, a `log.Panic`, or a `testing` helper that ends the goroutine
  counts. Only a direct call is recognized; a helper that ends the program on
  its own is not followed;
- it is the right operand of a `&&` or a `||`, which runs only when the left
  operand does not decide the result;
- it follows a `break`, a `continue`, or a `goto` that leaves the rest of the
  list. A labeled `break` or `continue` counts wherever it appears;
- it follows a `for` with no condition or a `select` with no case, neither of
  which finishes. An empty `select` blocks forever.

Mutual recursion between two functions is not examined. A call inside a
function literal, in the post statement of a `for`, through a function value,
or through an interface value runs without naming the function, so none of them
is examined either.

The shape is sometimes deliberate: a function that exists only to give a
profile an address, or a test that overflows the stack on purpose. It is
reported too.

The rule is adapted from revive's `unconditional-recursion`, which reports the
same message from a walk that resolves the callee by name and tracks no
`break`, `continue`, or `goto`.

Report only: ending the recursion means choosing what the function does
instead.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/uncondrecursion/cmd/uncondrecursion
go tool uncondrecursion ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -uncondrecursion ./...
```
