# lesiw.io/tools/analyzers/ineffectualcall

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ineffectualcall.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ineffectualcall)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call written as a statement to a method
with a value receiver whose body only returns an effect-free value.

```go
type point struct{ x int }

func (p point) X() int { return p.x }

func f(p point) {
    p.X() // call to X has no effect
}
```

The Go spec gives a method an extra receiver parameter
(https://go.dev/ref/spec#Method_declarations), and a call assigns the receiver
expression to that parameter (https://go.dev/ref/spec#Calls). A value receiver
is a copy. In statement context the result is not used, and the copy is
discarded, so a call to a method whose body is one return of an effect-free
value has no effect.

The rule is adapted from staticcheck's `SA4017`
(https://staticcheck.dev/docs/checks/#SA4017), which reports a discarded result
when the function has no effect other than the result.

## Checks

The call has to be an expression statement whose expression is a method call,
possibly parenthesized. A call in an init clause of an if, for, switch, or type
switch, and a call in the post clause of a for, count too.

The method is declared in the package under analysis and has a body and at least
one result. Its receiver type is not a pointer, and the receiver expression is
a value. A method expression such as `T.M(v)` and a method value stored in a
variable are not calls of this form. An immediately called method value such as
`(p.X)()` is a method call on a value and is reported.

The receiver expression and every argument have to be effect-free as well, or
the call can do work or panic.

The body is exactly one return statement. Each returned expression is one of:

- an identifier or a constant expression, including a len or cap of an array;
- a field selection that does not pass through a pointer, or a package-level
  variable, constant, or function;
- a parenthesized expression;
- an address, or a unary arithmetic, bitwise, or logical operation on a value;
- a binary addition, subtraction, or multiplication, a bitwise or logical
  operation, or a comparison, on basic values;
- a composite literal other than a map literal, of a type that is not a type
  parameter;
- a function literal;
- a conversion whose result type is not an array, a pointer to an array, or a
  type parameter.

A bare return with named results qualifies, since it returns the zero values.

Not reported, and why:

- a method with a pointer receiver, because a pointer receiver can change the
  caller's value;
- a call through a pointer, because the implicit dereference can panic on a nil
  pointer;
- a method promoted through an embedded pointer, because the selection
  dereferences the embedded pointer;
- an interface method, a method from another package, and a method with no body,
  because the package under analysis cannot see the body;
- a method whose body has more than one statement, even when every statement
  returns a value;
- a return expression with a non-constant call, a receive, a dereference, an
  index, a slice expression, a type assertion, a non-constant division,
  remainder, or shift, a map literal, a composite literal of a type parameter,
  a conversion whose result type is a type parameter, a method expression, a
  method value, or a conversion whose result type is an array or a pointer to
  an array;
- a receiver or argument expression that is not effect-free or can panic;
- a call whose result is used, assigned, or blank-assigned, and a call in a
  `defer` or `go` statement.

Report only: deleting the call can leave a variable or an import unused, which
breaks the build, or remove a comment with the statement.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ineffectualcall/cmd/ineffectualcall
go tool ineffectualcall ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -ineffectualcall ./...
```
