# lesiw.io/tools/analyzers/nilinterfacecmp

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilinterfacecmp.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilinterfacecmp)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of an interface value that
holds a typed nil with nil.

```go
func check() bool {
	var p *int
	var v any = p
	return v != nil // interface holds a typed nil, so the comparison is always true
}
```

Assigning a nil pointer, slice, map, channel, function, or `unsafe.Pointer`
to an interface stores the value's concrete type. The Go spec's Variables
section gives the example "x = v // x has value (*T)(nil) and dynamic type
*T". Its comparison operators section says two interface values are equal if
they have "identical dynamic types and equal dynamic values or if both have
value nil". The interface holds `*int` and the predeclared nil has no dynamic
type, so `v` is not nil and `v != nil` is true.

The analysis tracks a nil of concrete type from the assignment or conversion
that puts it in an interface to a comparison with the predeclared nil in the
same function. Declarations and assignments are read in source order, and the
paths into a block are joined, so a typed nil is reported only where every path
holds one. A call that cannot return, such as `panic` or `os.Exit`, ends its
path, so that path does not join.

These assignments supply a known nil:

- a variable declared without a value, as `var p *T` or a named result,
  which holds a nil;
- an explicit nil of a concrete type, as `p := (*T)(nil)`;
- the predeclared nil assigned to a variable of a nilable type;
- a conversion to an interface, as `error(p)` or `any(p)`, whose argument is a
  nil of a concrete type;
- a conversion to a concrete nilable type, as `S(p)` for a defined slice type,
  whose argument holds a nil;
- an assignment from one variable to another, which carries the same dynamic
  type and value.

The comparison is with the predeclared nil on either side, and the operator is
`==` or `!=`. Parentheses around the operands do not change the result.

Exempt, deliberately:

- A comparison of a concrete pointer, slice, map, channel, function, or
  `unsafe.Pointer` with nil. Its static type is not an interface, and the
  comparison tests the value for nil.
- An interface variable that holds a nil interface, as `var err error` or
  `err = nil` does. The comparison with nil is the intended test.
- A value a call returns, even when the call is in the same package and returns
  a nil of concrete type. Only assignments written in the function are read;
  another function's results are not.
- A parameter of the function. Its value comes from the caller, so it is not a
  known nil until the body assigns it.
- A value read from a field, an element, a type assertion, or a receive. Those
  expressions supply no known nil, whether or not they are assigned to a
  variable.
- A comparison with a typed nil, as `err == (*T)(nil)`. The nil operand has to
  be the predeclared nil.
- A variable of a type parameter type. Its zero value depends on the
  instantiation, so the comparison is left alone.
- A package-level variable, a variable whose address is taken, including by a
  pointer-receiver method call, and a variable a function literal writes.
  Another function or goroutine can change those, so their values are not
  fixed.
- A comparison inside a function literal that reads a variable declared
  outside it. The literal is analyzed as a separate function, and only
  variables it declares are followed.
- A `case nil` in an expression switch or a type switch. Only a comparison
  written with `==` or `!=` is reported.
- A comparison outside a function body, as in a package-level variable's
  initializer. The rule reads function bodies.
- A comparison inside the target expression of a `range` statement with `=`,
  or inside the left side of a select communication assignment. The rule does
  not follow the values those statements assign.

Report only: the result of the comparison is fixed, and a repair would change
what the code tests.

The rule is adapted from staticcheck's SA4023, which also reads the nilness of
function results.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilinterfacecmp/cmd/nilinterfacecmp
go tool nilinterfacecmp ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilinterfacecmp ./...
```
