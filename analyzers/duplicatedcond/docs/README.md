# lesiw.io/tools/analyzers/duplicatedcond

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/duplicatedcond.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/duplicatedcond)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if whose condition repeats the condition
of the if it is nested in.

```go
func f(ready bool) {
    if ready {
        if ready { // condition repeats the condition of the if it is nested in
            println("ready")
        }
    }
}
```

The rule reports when the inner if is the first statement of the outer body,
has no init statement, and the condition reads the same value both times. By
the if statements section of the spec, the block is executed when the condition
is true. No statement runs between the two tests, so the inner test is true
whenever the outer body runs.

A statement before the inner if can change a value the condition reads, an init
statement can change one before the inner condition is evaluated, and a call
with an effect or a receive can give a different value the second time. The
length of a channel can change when another goroutine sends to or receives from
it, and a type parameter whose type set may hold one has the same problem, so
those length reads are left alone. An else if is reached only after the first
condition was false, so it cannot repeat a test that passed.

Report only: the tool cannot tell which test was meant.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/duplicatedcond/cmd/duplicatedcond
go tool duplicatedcond ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -duplicatedcond ./...
```
