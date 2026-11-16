# lesiw.io/tools/analyzers/errshadow

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errshadow.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errshadow)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a declaration directly in an if body that
shadows an error variable the condition tested.

```go
func fetch() error {
    if err := read(); err != nil {
        err := repair() // declaration shadows the error variable "err"
        return err
    }
    return nil
}
```

The declarations and scope section of the spec says: "An identifier declared
in a block may be redeclared in an inner block. While the identifier of the
inner declaration is in scope, it denotes the entity declared by the inner
declaration." The body of the if is an inner block, so a later use of `err`
denotes the inner variable.

The shadowed variable has to be one whose type implements `error`. The inner
declaration can have any type. A type parameter is not treated as an error
variable, because the behavior of `types.Implements` is unspecified for an
uninstantiated generic type. The declaration can be a short variable
declaration or a `var`
declaration. It has to be the only non-blank variable the statement declares.

Any declaration not directly in the if body is left alone, including one in a
nested bare block, a plain else block, or a loop header. An else-if body and a
nested if body are if bodies, so they are checked when their condition tests
the error. A declaration that introduces another variable is left alone. A
declaration in a body whose condition requires the error to be nil through
direct nil comparisons and the boolean operators is also left alone, since the
body runs with a nil error.

The report does not depend on how the body uses the declaration. It reports the
shadow even when the initializer reads the outer error, as in `fmt.Errorf` with
`%w`, and even when the outer error is used after the if.

The rule is inspired by staticcheck's SA4006 family of forgotten-error checks.
The shadow lookup follows the shadow analyzer in golang.org/x/tools. That
analyzer also requires the two variables to have the same type and the outer
variable to be used after the declaration, and it ignores a self-redeclaration
such as `err := err`. This rule requires neither the type nor the later use,
and it reports the self-redeclaration.

Report only: an assignment changes which variable later statements use, and a
rename needs a name.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errshadow/cmd/errshadow
go tool errshadow ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errshadow ./...
```
