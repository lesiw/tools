# lesiw.io/tools/analyzers/selectsleep

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/selectsleep.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/selectsleep)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a select statement with a single case that
receives from `time.After` and discards the value.

```go
func wait() {
    // select with a single case that receives from time.After; write time.Sleep(time.Second)
    select {
    case <-time.After(time.Second):
    }
}
```

The `time.After` doc says "After waits for the duration to elapse and then
sends the current time on the returned channel"
([time.After](https://pkg.go.dev/time#After)), and the select statements
section of the Go spec says a select with no default "blocks until at least one
of the communications can proceed"
([Select statements](https://go.dev/ref/spec#Select_statements)). A select with
one case and no default therefore blocks for the duration and then discards the
time it receives, so `time.Sleep(d)` writes the same wait in one statement.

Not reported:

- a second case, including one that receives from `time.After` beside another
  channel;
- a `default` clause;
- a case that stores the received time, `case at := <-time.After(d):`;
- a `time` name that is not the imported package, such as a local variable of
  that name with an `After` method.

A single-case select that this rule leaves alone is `selectone`'s report,
which removes the select.

Fixable: the select is replaced with `time.Sleep(d)`, written with the import
name the file uses for the `time` package, so an import named `t` yields
`t.Sleep(d)`, followed by the case body. A blank assignment discards the value
too, so `case _ = <-time.After(d):` is fixed.

A case with no body is replaced with the call alone, which keeps the argument
list as written: a comment among the arguments and a trailing comment before the
closing parenthesis both survive. A comment anywhere else inside the select
withholds this fix.

A case with a body is fixed only when moving its statements into the block
around the select cannot change the program. The fix is withheld when a name in
the case is declared anywhere in that block or in a scope that block can see;
when the body holds a `break` that would leave a different statement; when it
holds a raw string that spans lines, which the reindentation would change; when
the select does not occupy whole lines; when a `goto` would jump over a
declaration the fix moves; and when a sibling select would move a name into
that block too, since the pair would redeclare it. The fix is withheld for a
dot import as well, where there is no import name to write. The select is still
reported in every one of those cases.

The rule is adapted from staticcheck's `S1037`, which reports a select that
receives from `time.After` and offers a fix for the empty case.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/selectsleep/cmd/selectsleep
go tool selectsleep ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -selectsleep ./...
go tool lesiw.io/tools/cmd/fix -selectsleep ./...
```
