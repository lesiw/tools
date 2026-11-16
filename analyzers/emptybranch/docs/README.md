# lesiw.io/tools/analyzers/emptybranch

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/emptybranch.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/emptybranch)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if or else branch with no statements
and no comment.

```go
func check(ready bool) {
    if ready {
    } // empty if branch
}
```

The Go spec says a block is "a possibly empty sequence of declarations and
statements and nested labeled statements" (https://go.dev/ref/spec#Blocks), so
the compiler accepts an empty branch. An empty branch is often a placeholder
for code not yet written, or a leftover from an edit.

An else block is reported the same way:

```go
func check(ready bool) {
    if ready {
        serve()
    } else {
    } // empty else branch
}
```

The rule reports each branch separately, so an if with two empty branches gets
two diagnostics. The spec's production is
`IfStmt = "if" [ SimpleStmt ";" ] Expression Block [ "else" ( IfStmt | Block ) ] .`
(https://go.dev/ref/spec#If_statements). The else operand is
`( IfStmt | Block )`: when it is an `IfStmt`, there is no else block to report,
and that statement's branches are checked as an if.

A comment between the braces suppresses the report, since it may explain why
the branch is empty:

```go
if ready {
    // Nothing to do until the queue drains.
}
```

Only a comment between the braces counts, since a comment elsewhere may explain
the condition or the code around the statement rather than the branch. A
comment before the condition, inside it, between the condition and `{`, between
the branches, between `else` and `{`, or after the closing brace does not
suppress the report.

Not reported:

- a branch with a statement;
- a branch with a comment between its braces;
- an empty for, switch, or select block, or an empty case or comm clause body.

Report only: either branch may be a placeholder for code not yet written, so no
rewrite is offered.

The rule is adapted from staticcheck's SA9003
(https://staticcheck.dev/docs/checks/#SA9003). SA9003 reports empty branches,
including one that holds only a comment; this rule leaves the commented branch
alone. SA9003 also skips an empty then branch when the else has a body or is an
`else if`; this rule reports it.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/emptybranch/cmd/emptybranch
go tool emptybranch ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -emptybranch ./...
```
