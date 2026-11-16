# lesiw.io/tools/analyzers/panicnil

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/panicnil.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/panicnil)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to panic with the predeclared nil.

```go
func at(xs []int, i int) int {
    if i < 0 || i >= len(xs) {
        panic(nil) // a nil panic gives no reason for stopping
    }
    return xs[i]
}
```

The call passes nil, so the crash report can say only that the argument was
nil.

The `builtin` package documents the behavior
(https://pkg.go.dev/builtin#panic): "Starting in Go 1.21, calling panic with a
nil interface value or an untyped nil causes a run-time error (a different
panic). The GODEBUG setting panicnil=1 disables the run-time error." With the
run-time error the report reads `panic: runtime error: panic called with nil
argument`; with `panicnil=1` it reads `panic: nil`. The default follows the go
directive of the module that builds the program, so a module that declares Go
1.20 or earlier keeps the older behavior, where a deferred `recover` returns
nil, the value it also returns under a `runtime.Goexit`.

The rule reads the argument expression and does not evaluate it, so only the
predeclared nil is reported. These shapes are left alone:

- a variable, including one that holds a nil error value or is named nil;
- a conversion to an interface type, such as `panic(any(nil))`, which reaches
  the same run-time error;
- a conversion to a pointer type, such as `panic((*int)(nil))`, which panics
  with a typed nil;
- a call that names something other than the builtin panic, such as a
  declaration named panic or a method of that name.

The suite's `nilness` pass also reports a panic whose value its flow analysis
proves nil, so a `panic(nil)` gets a second diagnostic when both analyzers run.
This rule reports the literal at every call site, including a deferred call and
a closure in a package-level initializer, which `nilness` leaves alone.

Report only: the repair is a message that states why the program stopped, which
the analyzer cannot supply.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/panicnil/cmd/panicnil
go tool panicnil ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -panicnil ./...
```
