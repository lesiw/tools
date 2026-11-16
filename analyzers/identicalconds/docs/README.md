# lesiw.io/tools/analyzers/identicalconds

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/identicalconds.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/identicalconds)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an else-if that repeats an earlier
condition in the same chain.

```go
func f(n int) int {
    if n > 0 {
        return 1
    } else if n > 0 { // else-if repeats an earlier condition
        return 2
    }
    return 0
}
```

The Go spec says an if statement executes its if branch when the expression is
true and, when present, its else branch otherwise
(https://go.dev/ref/spec#If_statements). Every else-if is reached through the
else branch, so every condition before it is false there. A repeat is reported
only when no init statement or value-changing condition lies between the two.
The earlier condition is false at the later test, and nothing between them can
change the value it read, so the repeated condition is false too and the branch
can never run.

The rule compares a condition with the conditions since the last init or
effectful condition, and reports every repeat of a recorded condition. It
compares the printed text of the conditions. Parentheses around the whole
condition are ignored, and line breaks are ignored. Parentheses inside it
remain in the text, so `(n) > 0` does not match `n > 0`. Every pure condition
is recorded, including one that contains an earlier condition: after `a`,
`a && b` is recorded but is not a repeat; a later `a && b` is reported.

An init statement ends the comparison: it can change a variable that an earlier
condition used, or declare a name that shadows one, so conditions before it are
not compared with conditions after it. A conversion, a type assertion, and a
call to `cap`, `complex`, `imag`, `len`, `max`, `min`, or `real` are pure in
themselves, but an effectful argument or operand still ends the comparison. Any
other call, and a channel receive, ends it too: the condition is not recorded,
and the earlier conditions are cleared. `len` also ends it when its argument
can be a channel, since another goroutine can send to or receive from it. A
condition whose `&` operand contains a composite literal ends the comparison:
the address may belong to a fresh allocation and differ between evaluations. A
function literal runs no code when it is created, so nothing in its body ends
the comparison.

An else block ends the chain. A chain inside the block is checked on its own,
so its conditions are not compared with the outer chain.

Report only: removing the branch would drop any comment inside it, and a
repeated condition usually means one condition was mistyped. The intended
condition is not in the source.

The rule is adapted from revive's identical-ifelseif-conditions.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/identicalconds/cmd/identicalconds
go tool identicalconds ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -identicalconds ./...
```
