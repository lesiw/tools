# lesiw.io/tools/analyzers/ifreturn

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ifreturn.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ifreturn)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an else block after an if whose body only
returns.

```go
func sign(n int) int {
    if n < 0 {
        return -1
    } else { // else follows an if whose body only returns
        return 1
    }
}
```

Go's Code Review Comments give the shape as "indent error flow"
(https://go.dev/wiki/CodeReviewComments#indent-error-flow). The body returns
when the condition holds, so the else runs only when it does not. The else
block can follow the if at the same indent, so its statements lose a level.

```go
func sign(n int) int {
    if n < 0 {
        return -1
    }
    return 1
}
```

The body must be a single return. A body that does other work before returning
is left alone; errflow reports that shape. An else-if chain is not reported.
Dropping a link's else moves the block out of the chain, and dropping the final
else can run it after an earlier branch that did not leave the function. An if
with an init statement is not reported, because the else block can name a value
declared there. An empty else block, an else block that holds only a comment,
and an if with no else are not reported.

## Fixes

The fix removes the else and its braces and outdents the block. It is withheld
when a comment sits between the closing brace of the if body and the opening
brace of the else block, since the edit would delete the comment. It is also
withheld when the else block holds a raw string that spans lines, since its
indentation is part of the value.

Outdenting a declaration changes its scope. The fix is withheld when the else
block declares a name that a later statement in the same block uses, declares,
or assigns. A short declaration can reuse a name that the surrounding block
declared before the if, or that an earlier declaration in the else block
introduced, when the name is a variable of the same type, the declaration also
introduces a new name, and nothing before or inside the if has taken the
address of that name, taken a slice of it when it is an array or a type
parameter, captured it in a closure, or made a method value from it. A var,
const, or type declaration in the else block cannot reuse the name. When the if
sits directly in the function body, the fix is also withheld when the name
belongs to a parameter, result, receiver, or type parameter. A declaration that
hides a named result also withholds the fix when a naked return follows, since
the return would no longer see the result. When the if sits directly in a type
switch or select clause, the fix is withheld when the name belongs to the guard
or receive variable. The fix is also withheld when the else block declares a
name and the surrounding block contains a goto; the label's position is not
checked. A blank variable declaration withholds it there too.

The rule is adapted from revive's indent-error-flow. The suite's errflow
reports the same else without a fix; this rule supplies the fix for the body
that only returns. A run of the whole suite reports the else twice.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ifreturn/cmd/ifreturn
go tool ifreturn ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -ifreturn ./...
go tool lesiw.io/tools/cmd/fix -ifreturn ./...
```
