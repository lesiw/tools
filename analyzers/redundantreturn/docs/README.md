# lesiw.io/tools/analyzers/redundantreturn

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/redundantreturn.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/redundantreturn)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a return statement with no results at the
end of a function that has no results.

```go
func f() {
    return // return statement at the end of a function with no results has no effect
}
```

The [function declarations](https://go.dev/ref/spec#Function_declarations)
section of the spec requires a body's statement list to end in a terminating
statement only when the signature declares result parameters. The [defer
statements](https://go.dev/ref/spec#Defer_statements) section says a function
also returns by reaching the end of its body. A function without a result type
returns when its last statement finishes, with or without a trailing return, so
the return has no effect.

Deleting a return that another statement follows would let that statement run,
so only the statement that ends the body is reported. A return nested in a
final if, switch, select, or block is left alone, as in S1023, and a return
inside a loop is left alone because deleting it would let the loop run again.
A function literal is a function, so a trailing return in one is reported. A
function that declares result parameters is left alone. A result list with no
parameters declares no results, so that form is reported. A labeled return is
not reported: the labeled statement is last, and deleting the return would
leave the label without a statement.

A generated file, including cgo output, is left alone, since an edit would
change generated text instead of its source.

The rule is adapted from the return check in staticcheck's S1023.

Fixable: the return is deleted. The line is deleted with the return when the
return is alone on the line and the line ends with a newline; otherwise the
return and a semicolon that follows it after spaces or tabs are removed, so a
comment or the text before the return stays.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/redundantreturn/cmd/redundantreturn
go tool redundantreturn ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -redundantreturn ./...
go tool lesiw.io/tools/cmd/fix -redundantreturn ./...
```
