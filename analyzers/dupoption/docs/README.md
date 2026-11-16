# lesiw.io/tools/analyzers/dupoption

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dupoption.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dupoption)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an option that a later argument repeats.

```go
func newAlpha() *server {
    return newServer("alpha",
        withName("alpha-one"), // argument is repeated later
        withTimeout(time.Second),
        withName("alpha-one"),
    )
}
```

A function applies each option it is handed. An option that sets a field of the
value it is handed is overwritten by a later one, so the earlier assignment is
dead. An option that appends to a list adds a second entry to it.

The rule examines an argument in a variadic tail whose element type is a
function that takes one parameter or more and returns nothing or one error, as
in `newServer(name string, opts ...option)` with `option` declared as
`func(*server)` or `func(*server) error`: an option configures the value it is
handed. It cannot tell an option from a callback list, so a list such as
`steps ...func(*testing.T)` is examined too. A `func()` element is handed
nothing to configure, an interface element is a value rather than a function,
and a function that returns anything else transforms what it is given, so a
repeat there composes, and those tails are left alone. A call that spreads its
tail, as `newServer(name, opts...)` does, has one argument in the tail, and one
that supplies its parameters from a single multi-value call, as
`build(settings())` does, has one expression for the whole list, so neither has
a pair to compare. A builtin is not examined: `append` takes such a tail, but
it adds each function to the list instead of applying it, so
`append(chain, m, m)` is left alone. Only the arguments in the tail are
compared with each other, so a fixed parameter repeated in the tail is left
alone.

Two arguments that are not calls are the same option when they print the same
and reading one has no effect. A pair of calls is the same option when their
function expressions and their arguments pass that test. A call that names one
option twice reads as if two different options were meant, so the repeat is
reported even when applying it twice was intended. So
`withTimeout(2*time.Second)` twice is a repeat, while `withTimeout(wait())`
twice is not, since reading the argument a second time may give a different
value. A repeated option with different arguments may be deliberate, as with
several tags, so the rule leaves it alone. A pair of function literals is left
alone, since writing a literal twice names no option. Three copies of one
option produce two diagnostics.

The rule cannot see what a function does: it assumes the options are applied to
one value in order, that a call returns the same option each time, and that
nothing between the pair writes what they read, so `withNextName()` twice is
still reported.

The idea comes from go-critic's dupOption, which compares only the printed text
of each argument.

Report only: removing one of the two could change behavior.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dupoption/cmd/dupoption
go tool dupoption ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dupoption ./...
```
