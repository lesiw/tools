# lesiw.io/tools/analyzers/timenowloop

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/timenowloop.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/timenowloop)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `time.Now` call in the condition of a
for statement.

```go
for time.Now().Before(deadline) { // time.Now is called in the loop condition
    ready()
}
```

The spec says of the for statement: "The condition is evaluated before each
iteration" (https://go.dev/ref/spec#For_statements). A time.Now call there can
run on each iteration.

A call directly in the loop body, the init statement, or the post statement is
left alone. A variable that contains the result and is used in the condition
is also left alone, as is a condition that calls a helper which calls
time.Now.

A function literal in the condition is searched for direct time.Now calls when
the condition calls it in place. A literal passed to another function is left
alone for direct time.Now calls, because that function may call it later or
never. A literal started with `go` is left alone for direct time.Now calls,
because the goroutine is not part of the condition's evaluation. A deferred
literal is searched, because it runs before the enclosing in-place call or
literal returns. Every for condition is checked, including one nested in any
literal; arguments to a go call are evaluated in place and are also checked.

A literal reached through a variable or returned by another call is not
followed.

A call started with `go` is left alone; the rest of a go statement, including
its function expression and arguments, is evaluated in place and checked.

Only a direct call to `time.Now` is reported, through a package selector or a
dot import. A variable assigned `time.Now` and a method named `Now` on another
type are left alone.

There is no fix. Moving the call ahead of the loop changes when it runs.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/timenowloop/cmd/timenowloop
go tool timenowloop ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -timenowloop ./...
```
