# lesiw.io/tools/analyzers/looponce

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/looponce.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/looponce)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a loop that runs at most once.

```go
func f(n int) int {
    for { // loop runs at most once
        if n > 0 {
            return n
        }
        return 0
    }
}
```

The For statements section of the Go spec says a `for` statement evaluates its
condition before each iteration (https://go.dev/ref/spec#For_statements). A
body that ends in a return or a break that ends the loop ends on its first
iteration, so the condition is not evaluated again.

## Checks

A loop is reported when its body has at least two statements, one of them a
control statement at the top level, and ends in a return or a break that ends
the loop. The control statement is an `if`, a loop, a `switch`, a type switch,
or a `select`. The rule inspects only the top-level statements of the body to
find the control statement and the exit; a nested loop, switch, or select is
not followed to see whether it finishes.

```go
func f(n int) int {
    for i := 0; i < n; i++ {
        if i > 0 {
            return i
        }
        return 0
    }
    return -1
}
```

A break at the end of the body leaves the loop whether or not it has a label.
A label on the break names the statement that the break leaves, which may be
this loop or a statement that encloses it.

A body that ends another way is left alone even when every path through it
leaves, as when an `if`/`else` returns in both arms.

## Exemptions

A body of one statement is left alone, since the rule requires the control
statement and the exit as separate top-level statements. A range body of one
statement is the first-element idiom:

```go
func f(xs []int) int {
    for _, x := range xs {
        return x
    }
    return 0
}
```

A body with no top-level control statement is left alone; the rule requires
one, as SA4004 does.

A `goto` in the loop body, other than one inside a nested function literal,
suppresses the report, since it may reach the exit or skip it. A `continue`
suppresses the report when it repeats the checked loop, either without a label
or with a label that names the loop. An unlabeled `continue` in a nested loop
repeats that loop and does not suppress the report. A `continue` whose label
names a loop that encloses this one leaves this loop and does not suppress the
report.

A range over a map is left alone, since one pass is a plain way to take an
arbitrary element. A range over a function is left alone, since the rule does
not inspect the iterator. A range over a type parameter is left alone without
reading its type set, so this includes a type parameter whose set is only
slices.

## Fixes

A `for` loop becomes a block, or an `if` when it has a condition and no init.
An init statement becomes the first statement of the new block, and a condition
moves into an `if`. The body's statements stay in a block. A block that
declares nothing is removed by `unnecessaryblock` on the next fix round. An
unlabeled final break is removed, and a post statement is dropped, since the
body leaves before it can run. The fix is withheld when a
break targets the loop anywhere other than an unlabeled final break, when a
comment sits in the part of the loop that the edit replaces or on an unlabeled
final break, or when dropping the post would leave a variable declared in the
function body, an import name, or an object from a dot import with no reader
outside another loop post.

A range loop is report only, since a replacement would need a guard and a
first-value expression that differs for each kind of rangeable value.

The rule is adapted from staticcheck's SA4004.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/looponce/cmd/looponce
go tool looponce ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -looponce ./...
go tool lesiw.io/tools/cmd/fix -looponce ./...
```
