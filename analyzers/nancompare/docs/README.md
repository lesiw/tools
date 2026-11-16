# lesiw.io/tools/analyzers/nancompare

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nancompare.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nancompare)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison with `math.NaN()`.

```go
func stopped(v float64) bool {
    return v == math.NaN() // no value equals NaN; this is always false; write math.IsNaN(v)
}
```

The math package defines both functions: "NaN returns an IEEE 754
'not-a-number' value" and "IsNaN reports whether f is an IEEE 754
'not-a-number' value." The spec compares floating-point values "as defined by
the IEEE 754 standard". Under that standard a NaN is unequal to every value,
including itself, and every ordering comparison with a NaN is false. `==`, `<`,
`<=`, `>`, and `>=` are false when either operand is a NaN, and `!=` is true,
so each comparison has a constant answer.

The rule reports all six comparison operators. It also reports a case in an
expression switch that compares the tag with NaN, as `case math.NaN():`; the
case can never match. A switch whose tag is `math.NaN()` is reported as well:
no case can match. A side is read as NaN when it is a call to `math.NaN()` or a
conversion of one to a floating-point type: `float32(math.NaN())` and
`temperature(math.NaN())` are read.

The fix writes `math.IsNaN(x)` for `==` and `!math.IsNaN(x)` for `!=`, using
the name the file gives the math package. An operand that is not assignable to
`float64` is converted, so a `float32` or a defined float type is written as
`float64(x)`; an untyped constant and a `float64` are passed as they are. The
fix changes the answer for a NaN operand.

The fix covers `==` and `!=`, where `math.IsNaN` is the test the comparison
was meant to make. The ordering operators are reported without a fix:
`math.IsNaN` does not have the same answer, and a `false` constant would drop
evaluation of the other operand, which may have effects. A switch case is
reported without a fix as well; a fix would have to replace the case with a
test of the tag, and the switch is not rewritten.

The fix is withheld where the other operand is not assignable to `float64` and
is not a float type (an interface, for example, which `IsNaN` does not take),
where both sides are NaN, where the file dot-imports math, where the comparison
spans lines or holds a comment, where the operand does not print on one line,
where the rewrite would take the line past the width, and where a name the
rewrite writes reaches another object. The rewrite is withheld in those cases
because lines would be joined, the comment would be dropped, or the wrong name
would be written. The diagnostic is still reported.

Only the call or a conversion to a concrete float type is read. A NaN held in
a variable is left alone; values are not tracked across statements. A
conversion to a type parameter, as `T(math.NaN())` where `T` is a float type
parameter, is left alone because the conversion target is not a concrete type.
`-math.NaN()` and a complex construction such as `complex(math.NaN(), 0)` are
left alone as well; in those forms the call is inside a larger expression.
staticcheck's SA4012 reports a NaN held in a variable through SSA; this rule
does not.

The rule is adapted from staticcheck's SA4012.

Fixable: the equality forms.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nancompare/cmd/nancompare
go tool nancompare ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nancompare ./...
go tool lesiw.io/tools/cmd/fix -nancompare ./...
```
