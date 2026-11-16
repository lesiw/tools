# lesiw.io/tools/analyzers/badsyncfunc

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/badsyncfunc.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/badsyncfunc)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `sync.Once.Do` call whose function reads
a variable that a loop enclosing the call declares.

```go
func f(ready *sync.Once, names []string) {
    for _, name := range names {
        ready.Do(func() { use(name) }) // sync.Once.Do runs the function once, so one iteration's name is used
    }
}
```

`sync.Once`'s doc says that "a new instance of Once is required for each
function to execute", and that "if once.Do(f) is called multiple times, only
the first call will invoke f, even if f has a different value in each
invocation." A loop that passes a new function on every iteration and reuses
the Once invokes the first iteration's function and no other.

A `defer` or `go` between the loop and the call does not change the report: the
function still runs for one of the calls and the other calls do not invoke it.
Nor does an assignment or a call inside such a function give the iteration a
fresh Once.

The call may be on a Once value, on a Once field, on an embedded Once, or
written as the method expression `(*sync.Once).Do(&once, f)`.

Not reported:

- a `Do` outside a loop, and a `Do` on a Once that the loop builds: one a loop
  body declares, or one a range clause binds, unless another loop binds a
  pointer;
- an assignment that writes the Once, a value holding it, or the field a
  promoted `Do` is reached through;
- a call in the loop given a pointer to the Once, as in `reset(&once)` or
  `reset(once)`, though a method on the value that holds it, as in `b.Reset()`,
  is reported;
- a literal that reads no variable a loop declares, such as a package-level
  variable or a constant, or one that only writes a variable the loop
  declares;
- a variable declared outside the loop, even one the loop assigns;
- a variable that the function literal declares, or that a function body
  between the loop and the call declares;
- a struct field, including one of a type the loop declares;
- an argument that is not a function literal, as in `load.Do(maker(name))`;
- a receiver that is not a variable, such as `getOnce().Do` or `onces[i].Do`;
- a `Do` that another type declares, and a method value such as
  `do := load.Do; do(f)`.

A `Do` written to keep the first iteration's value is reported too.

Report only: the repair is either to move the `Do` call out of the loop for the
one value it should use, or to create the Once inside it. Either edit changes
what the program does.

The rule is inspired by go-critic's `badSyncOnceFunc`, which reports a
`sync.OnceFunc` value that is discarded or called where it is built.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/badsyncfunc/cmd/badsyncfunc
go tool badsyncfunc ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -badsyncfunc ./...
```
