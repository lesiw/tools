# lesiw.io/tools/analyzers/errwrapdirect

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errwrapdirect.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errwrapdirect)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `fmt.Errorf` call that wraps an error
without a message.

```go
func wrap(err error) error {
    return fmt.Errorf("%w", err) // error is wrapped without a message
}
```

The `fmt` package documentation says that a `%w` verb with an error operand
returns an error whose `Unwrap` method returns the operand
([fmt.Errorf](https://pkg.go.dev/fmt#Errorf)). The `errors` package
documentation describes `fmt.Errorf` with `%w` as the way to add context to an
error ([Errors](https://pkg.go.dev/errors#pkg-overview)). A format of exactly
`%w` adds no text, so the message is the operand's message and the result
unwraps to the operand.

Only a literal format string is examined, and its value has to be exactly `%w`.
A format with any other value is left alone, and so is a format held in a
constant or a variable. A call with a missing, extra, or spread argument is
left alone. The argument has to be assignable to `error`.

A `%w` verb formats its operand like `%v`, so an operand that implements
`fmt.Formatter` is formatted by the `Format` method. An argument whose static
type implements `fmt.Formatter` is left alone; a `Formatter` behind the
`error` interface is not known.

The fix replaces the call with the argument. When the argument's static type is
exactly `error`, the argument can stand wherever the call stood. An argument
of any other type is converted to `error`, so the replacement has the call's
static type; the fix is withheld where `error` is shadowed. A unary argument
that would bind to a selector or a type assertion is parenthesized. The fix is
also withheld in an expression statement, a `go` statement, and a `defer`
statement, where an error value is not a statement. It is withheld where the
call is directly compared with `==` or `!=`, directly used as a switch tag or
case, directly used as a map key, or in the init statement of an `if`, `switch`
(including a type switch), or `for`. A conversion, a type assertion, and a
composite literal are followed. A comparison can panic when both operands hold
the same non-comparable dynamic type, which the replacement can create.

The replacement is not the same value. A comparison with `==` that was false
for the wrapper can be true for the operand, and the result has one fewer
`Unwrap` layer. A type assertion, `%T`, or `reflect.TypeOf` reports the
operand's type where it reported the wrapper's. A wrapper around a sentinel
may be deliberate for exactly that reason. The rule reports the call anyway.

A nil literal and a conversion of nil are left alone. Wrapping nil gives a
non-nil error, and a typed nil can make the operand's `Error` method panic. Any
other expression is not inspected for nilness, so an error variable that is
nil at run time reports and the fix returns nil. A comment in the text the fix
removes withholds the fix. A call in a test file is checked like any other
call; `//ignore` on the call keeps one a test needs.

The rule is adapted from staticcheck's S1028.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errwrapdirect/cmd/errwrapdirect
go tool errwrapdirect ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errwrapdirect ./...
go tool lesiw.io/tools/cmd/fix -errwrapdirect ./...
```
