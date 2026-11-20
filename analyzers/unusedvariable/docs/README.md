# lesiw.io/tools/analyzers/unusedvariable

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unusedvariable.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unusedvariable)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a variable that is declared and never read.

```go
func f() {
	v := "a" // variable v is declared and never read
}
```

The compiler already rejects the package, so the analysis exists for the fix,
which the compiler does not offer. It needs a tool that loads the packages
itself: the go command's `go vet` never hands one a package that does not
build.

A statement that only declares the name is removed whole, and a comment
sharing its line, before or after it, goes with it. A comment on a line of its
own above the statement stays, because it may describe the block rather than
the statement. Inside a `var` group a spec's comment is the spec's, so it goes
with the spec; it stays when only one of the names the spec declares goes.
When the right side has an effect, the value stays and only the name becomes
the blank identifier. The fix also removes every write to a variable the code
writes and never reads.

An expression that only computes a value is dropped with the name, so an index
or a dereference goes too. That costs nothing here: the package does not build
until the variable is gone, so the expression never ran.

A name that a type switch's guard or a select clause's receive declares keeps
the expression and loses the name, so `switch v := x.(type)` becomes
`switch x.(type)`, and `case v := <-ch:` becomes `case <-ch:`.

A range clause is left alone. The clause declares the name, so there is no
declaration statement to remove, and this rule reports only what it repairs. A
package whose remaining error is that one cannot be repaired, and `cmd/fix`
ends with the package that did not build.

Other checks report what a removal leaves behind: `emptylines` a blank line at
the edge of a block, `emptybranch` a branch with nothing in it, `discardedpure`
a call whose result nobody reads, and `simplifyrange` a `for` clause left with a
blank key.

This mirrors gopls' `unusedvariable`
(`gopls/internal/analysis/unusedvariable`, `golang.org/x/tools/gopls`), which
is internal and cannot be imported. The rule is that analyzer's; the deletion
and the effect test are adapted from x/tools' `refactor` and `typesinternal`
packages, and the deletion goes further, removing the writes with the
declaration.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unusedvariable/cmd/unusedvariable
go tool unusedvariable ./...
go tool unusedvariable -fix ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unusedvariable ./...
go tool lesiw.io/tools/cmd/fix -unusedvariable ./...
```
