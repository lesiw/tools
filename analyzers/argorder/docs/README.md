# lesiw.io/tools/analyzers/argorder

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/argorder.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/argorder)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call whose arguments name the parameters
in the other order.

```go
func join(from, to string) string { return from + to }

func f(from, to string) string {
    return join(to, from) // arguments are in the order to, from; the parameters are from, to
}
```

A call that transposes two parameters of the same type builds, so the argument
names are the only thing in the call that orders the values.

Both names must agree: the first argument is written with the second
parameter's name and the second with the first's. `join(b, from)` is not
reported, because `b` is not a parameter name.

The pair must be adjacent. `span(out, hi, lo)` for `span(lo, hi, out int)`
reverses all three arguments and reports nothing, because no adjacent pair is
transposed.

The two parameters must have the same type. A pair of assignable but different
types, such as a slice and a named slice type, is left alone.

Only a bare identifier is read as a name, and parentheses around one are
dropped: `join((to), (from))` reports. A literal, a call, a selector and an
index expression are left alone, because a field name says nothing about which
parameter it fills.

One multi-value expression can supply several parameters, as `join(pairOf())`
does, so the pairs stop at the shorter of the two lists.

The call may be a function, a method, a method expression, an interface
method, a func-typed value, or a func literal. A method expression's receiver
is the first argument and is compared like any other parameter.

A predeclared function's signature has no parameter names, and a function type
may have none either, so a call to one reports nothing. A call through a type
parameter is left alone.

A variadic parameter takes any number of arguments, so only the parameters
before it are read as pairs. `pack(last, first)` for `pack(first, last
string, more ...string)` reports, and `pack(first, last, last, first)` is not
reported, because only the fixed parameters are read as a pair.

The rule is like go-critic's `argOrder`, which reports a constant written
where one of the operands belongs; this rule compares the parameter names
instead.

Report only: swapping the arguments changes behavior, so there is no fix.

The rule compares names, so it reports three shapes: a transposition, a call
that reverses an ordering on purpose, and a wrapper whose parameters are named
the other way. Naming either argument something else clears the report.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/argorder/cmd/argorder
go tool argorder ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -argorder ./...
```
