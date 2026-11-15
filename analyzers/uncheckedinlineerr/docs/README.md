# lesiw.io/tools/analyzers/uncheckedinlineerr

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/uncheckedinlineerr.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/uncheckedinlineerr)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an empty body in an if that checks an error
from its init.

```go
func write(w io.Writer, src string) error {
    if _, err := io.WriteString(w, src); err != nil { // err is checked in an if with an empty body
    }
    return nil
}
```

The [Code Review
Comments](https://go.dev/wiki/CodeReviewComments#handle-errors) say to check an
error, then "handle the error, return it, or, in truly exceptional situations,
panic".

The condition may mention the error in any form, and the rule does not read the
sense of the comparison: `err != nil`, `err == nil`, or `err.Error() != ""`.

A body that contains a statement is left alone. Recording that the call failed
is legitimate.

A comment exempts the branch it sits in when it starts on a line of its own. A
comment that starts on the opening brace's line does not exempt the branch. The
rule does not read the text, so a `// TODO` exempts the branch just as an
explanation does.

With `err == nil` the branches are inverted, so an empty body can be the
success path and the `else` can hold the handling.

A statement or a comment in the `else` exempts the check, and an `else if` is a
statement. An empty `else` does not exempt the check.

The rule reads an identifier on the left of the init; an assignment to a field
or an element is left alone. It also leaves a conversion on the right alone,
since a conversion cannot fail. The init may declare the error or assign a
variable declared earlier, and the rule reads the statement alone, so a check
whose error the function returns after the `if` is still reported.

A value whose type implements `error` is an error for this rule. A pointer type
implements it where its value type does not, as `*net.AddrError` does. A type
parameter's method set comes from its constraint, and a type term adds no
methods, so a value whose static type is the parameter is an error only when
the constraint declares `Error`. A call that instantiates the parameter returns
a value of the concrete type, and the rule reads that type.

The idea comes from go-critic's `uncheckedInlineErr`, which reports an if whose
condition tests a different error than its init assigns, and from staticcheck's
`SA9003`, which reports an empty if or else body whatever the condition. This
rule reads the condition as well: it reports only when the condition mentions
the error the init assigns and the `else`, if any, holds nothing either.

Report only: a fix would have to write the handling, which the tool cannot
invent.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/uncheckedinlineerr/cmd/uncheckedinlineerr
go tool uncheckedinlineerr ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -uncheckedinlineerr ./...
```
