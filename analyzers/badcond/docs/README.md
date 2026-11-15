# lesiw.io/tools/analyzers/badcond

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/badcond.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/badcond)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a condition that is always true or always
false.

```go
func f(n int) {
    if n < 0 && n > 10 { // condition is always false
    }
    for i := 0; i > 0; i++ { // loop condition is false before the first iteration
    }
}
```

Each comparison has to put a constant against the operand, so `n < m && n > 10`
says nothing. `n < 0 && n > 10` asks for a value below 0 and above 10, so no
value satisfies it, and `n < 10 || n > 0` covers every value. Two bounds on the
same side, as in `n < 0 && n < 5`, still leave values that satisfy both.

The init statement of a `for` clause runs before the first test of its
condition, so a condition it makes false is false at the first test and the
body never runs. The init statement must assign a constant to one variable, and
a term of the condition must compare that variable with a constant, as in
`for i := 0; i > 0; i++`. `for i := 0; i > n; i++` is left alone, since a
negative `n` would run it. A condition the pair rule reports is not reported
again as a loop condition.

An operand whose second read can differ is left alone: a call can have an
effect, a receive consumes an element, and addressing a composite literal
returns a new value. A conversion, a type assertion, and the builtins `cap`,
`complex`, `imag`, `len`, `max`, `min`, and `real` give the same value twice. A
term with any effect between the two comparisons ends the scan, since a call
may write the shared operand, so `n < 0 && bump(&n) && n > 10` says nothing.

Floating-point values are compared "as defined by the IEEE 754 standard", and
a NaN fails every ordering comparison and is never equal to anything. A
tautology of ordering comparisons is reported only when the operand's type
cannot hold a NaN: a floating-point type, and a type parameter whose constraint
admits one, are left alone. A conjunction is reported whatever the operand's
type, since a NaN leaves it false. A pair of `==` and `!=` negates itself, so
one of the two is true for every value. For an interface operand, constants of
different types are different values, so `a == 1 && a != 1.0` says nothing.

`dupsubexpr` reports a value compared with itself, and `bools` reports
`n == 1 && n == 2`. This rule reports the `==`/`!=` pair on one constant, as in
`n == 1 && n != 1`, which neither covers.

Adapted from go-critic's `badCond`: the contradiction rule adds the disjunction
that covers every value, and the loop rule reports only a condition the init
statement makes false.

Report only: the tool cannot tell which condition was meant.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/badcond/cmd/badcond
go tool badcond ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -badcond ./...
```
