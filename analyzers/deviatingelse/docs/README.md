# lesiw.io/tools/analyzers/deviatingelse

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deviatingelse.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deviatingelse)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if/else whose else block does not fall
through to the statement after the if.

```go
func show(n int) {
    if n == 0 {
        print(n)
    } else { // else does not fall through; invert the condition and outdent the then block
        return
    }
}
```

The Code Review Comments say to keep the normal code path at the lowest
indentation (https://go.dev/wiki/CodeReviewComments#indent-error-flow). The
rule applies when the else block ends in return, break, continue, goto, a call
to panic, os.Exit, or a package-level Fatal or Panic function in log, or an if
whose two branches both leave the block. A labeled statement or a bare block
that ends in one of these forms counts too. Return, goto, and panic are
terminating statements (https://go.dev/ref/spec#Terminating_statements). A
break exits the innermost for, switch, or select, and a continue begins the
next iteration of the innermost for
(https://go.dev/ref/spec#Break_statements,
https://go.dev/ref/spec#Continue_statements). Inverting the condition puts the
else block in the if body and the then block after the if, which removes a
level of indentation.

An else-if chain is left alone, because inverting one link moves its block out
of the chain. An if with an init statement is left alone, because the then
block would leave the scope the init declares. An empty then block is left
alone, because it has nothing to outdent. A then block that ends in one of
those forms is left alone. errflow reports the return, break, continue, goto,
and panic cases, and ifreturn reports a then block that only returns. A then
block ending in a switch, select, or for is reported. For the else block, the
rule does not cover a switch, select, or for that never falls through, so those
are left alone.

## Fixes

The fix inverts the condition and moves the then block after the if. An
equality test swaps `==` and `!=`, a `!` that is the whole condition is
dropped, and any other condition is prefixed with `!`, with parentheses around
a binary condition.
The fix is withheld when the then block declares a name directly, because the
declaration would move into the enclosing scope. It is also withheld when the
then block holds a raw string that spans lines, since outdenting would change
its value, when either block's first statement shares the opening brace's line
or its last statement shares the closing brace's line, when a comment follows
the else block's closing brace on the same line, because the rewrite would move
it onto the last then statement, or when a comment would be removed: before or
after the condition, inside a parenthesized negated condition, between a
leading `!` and its operand, or between the then block and the else block.

The rule is inspired by revive's early-return.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deviatingelse/cmd/deviatingelse
go tool deviatingelse ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deviatingelse ./...
go tool lesiw.io/tools/cmd/fix -deviatingelse ./...
```
