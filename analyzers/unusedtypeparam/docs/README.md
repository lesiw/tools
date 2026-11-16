# lesiw.io/tools/analyzers/unusedtypeparam

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unusedtypeparam.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unusedtypeparam)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a type parameter a function, method, or
type alias never uses. The rule is original to this suite and follows the Go
spec's type parameter declarations and scope rules.

```go
func Count[T any](xs []int) int { // type parameter T is unused
    return len(xs)
}
```

The Go spec says each name in a type parameter list declares "a new and
different named type that acts as a placeholder for an (as of yet) unknown type
in the declaration", and that the type parameter "is replaced with a type
argument upon instantiation" ([Type parameter
declarations](https://go.dev/ref/spec#Type_parameter_declarations)). The scope
of a function's type parameter "begins after the name of the function and ends
at the end of the function body", and the scope of a type declaration's type
parameter "begins after the name of the type and ends at the end of the
TypeSpec" ([Declarations and
scope](https://go.dev/ref/spec#Declarations_and_scope)). Within that scope, a
type parameter counts as used when it occurs in the parameters, results, body,
or aliased type, or in the constraint of another type parameter that is used. A
constraint does not use the parameter it constrains.

In the signature `func Copy[T any, S ~[]T](s S) S`, `S` appears in the
parameters and result, so `S` is used and its constraint makes `T` used. In the
signature `func Only[T any, S ~[]T](n *int)`, neither `S` nor `T` appears in
the parameters or result, so both are reported even though `T` occurs in `S`'s
constraint. A constraint is not a use of the parameter it constrains, so the
signature `func Self[T interface{ ~[]T }](n int) int` reports `T`. In the
signature `func Keep[T any, _ ~[]T](n int) int`, `T` counts as used because the
blank slot remains and its constraint refers to `T`.

An alias denotes the same type as the one it aliases ([Alias
declarations](https://go.dev/ref/spec#Alias_declarations)). `P` is reported on
`type Alias[P any] = []int`, since `Alias[int]` and `Alias[string]` are both
`[]int`.

Not reported, and why:

- A type definition. The type argument is part of the instantiated type's
  identity, so `Tag[int]` and `Tag[string]` are different types even when no
  field has type `T` ([Type identity](https://go.dev/ref/spec#Type_identity)).
- A method receiver. The receiver type has one type parameter for each type
  parameter of the base type, and each appears in the receiver type
  ([Method declarations](https://go.dev/ref/spec#Method_declarations)). A type
  parameter list on the method itself is checked like a function's.
- A stub, with an empty body or a single statement that calls the builtin
  `panic`. Its signature may still change, as in `unusedparams`.
- The blank identifier, which is not reported. The parameters its constraint
  refers to still count as used, because the slot remains.

Report only: blanking is illegal when a constraint refers to the name, and
deletion changes the declaration and its callers or users. Either edit is left
to the author.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unusedtypeparam/cmd/unusedtypeparam
go tool unusedtypeparam ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unusedtypeparam ./...
```
