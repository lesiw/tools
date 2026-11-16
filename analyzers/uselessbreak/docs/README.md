# lesiw.io/tools/analyzers/uselessbreak

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/uselessbreak.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/uselessbreak)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a break at the end of a case clause in a
switch.

```go
func f(n int) {
    switch n {
    case 0:
        break // break at the end of a switch case has no effect
    }
}
```

The [break statement](https://go.dev/ref/spec#Break_statements) section of the
spec says a break ends the innermost for, switch, or select. The [switch
statements](https://go.dev/ref/spec#Switch_statements) section says the
statements of the selected case run and then the switch statement ends, so a
break at the end of a case leaves a switch that would end anyway. A break with
another statement after it skips that statement and is not reported. A
switch inside a loop is reported too: the break leaves only the switch, so it
does not leave the loop.

An expression switch and a type switch both have case clauses. The check is
limited to the statement that ends a case body, so a break inside a trailing if
or block, or a break inside a labeled statement, is not checked. A select clause
is not checked; select cases are outside this rule. A break at the end of a loop
body is not reported either: it ends the loop, so deleting it can run the next
pass.

The analyzer reports a bare break only, even when a label names the switch
itself. A labeled break can leave an enclosing statement, and deleting one
whose label has no other use would leave the label without a use, which the
compiler rejects. The analyzer skips every labeled break rather than decide
which statement the label names.

The rule is adapted from revive's useless-break, which also checks select
clauses.

Fixable: the break is deleted. The line is deleted with the break when the
break is alone on the line, and otherwise only the break is removed, so a
comment or the text before the break stays.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/uselessbreak/cmd/uselessbreak
go tool uselessbreak ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -uselessbreak ./...
go tool lesiw.io/tools/cmd/fix -uselessbreak ./...
```
