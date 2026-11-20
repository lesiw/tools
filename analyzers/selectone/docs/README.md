# lesiw.io/tools/analyzers/selectone

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/selectone.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/selectone)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a select statement with a single case
clause and no default.

```go
func wait(ch <-chan int) {
    select { // select with a single case is redundant
    case <-ch:
    }
}
```

The select statements section of the Go spec says a select with no default
"blocks until at least one of the communications can proceed"
([Select statements](https://go.dev/ref/spec#Select_statements)). With one
case, the send or receive in the case blocks the same way without the select.

Not reported:

- A select with no cases, a default clause, or more than one case.
- A select whose single case discards a value received from `time.After`, which
  the `selectsleep` analyzer reports.
- A select whose case body holds a break that leaves the select. The break
  statement section says a break ends the innermost for, switch, or select, or
  the statement named by a label
  ([Break statements](https://go.dev/ref/spec#Break_statements)). Removing the
  select would change where the break goes.

Fixable: the select is replaced with the send or receive in the case and the
body. The fix is withheld when it would delete a comment, when the select does
not occupy whole lines, when the body holds a raw string that spans lines, when
a name in the case is declared anywhere in the block around the select or in a
scope that block can see, when a goto would jump over a var declaration the fix
moves out of the case, and when a sibling select would move a name into that
block too.

A name in the case can shadow a name in an outer scope that later code in the
block uses, so moving the declaration would change which value that code reads.
An earlier declaration in the block makes a single-name short declaration a
redeclaration error, and a two-name receive assigns to the existing name. A
later declaration would collide with the moved name
([Declarations and scope](https://go.dev/ref/spec#Declarations_and_scope)).
The guard is conservative: it withholds the fix whenever the case declares a
name that resolves in a visible scope, even when no later code uses that name.
A goto before or inside the case that jumps to a label after the select must
not bring a variable into scope at its target
([Goto statements](https://go.dev/ref/spec#Goto_statements)), and moving a
declaration out of the case can do that. A blank `var _ = f()` counts: Go
rejects a jump over it even though it declares no variable.

The rule is adapted from staticcheck's S1000, which reports a single-case
select and offers no fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/selectone/cmd/selectone
go tool selectone ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -selectone ./...
go tool lesiw.io/tools/cmd/fix -selectone ./...
```
