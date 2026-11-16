# lesiw.io/tools/analyzers/discardedpure

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/discardedpure.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/discardedpure)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call whose only effect is its result,
written as a statement.

```go
func shout(name string) {
    strings.ToUpper(name) // result of strings.ToUpper is discarded; the call has no other effect
}
```

The "Expression statements" section of the Go spec allows a function or method
call in statement context (https://go.dev/ref/spec#Expression_statements). The
call computes a result and the result is discarded. When the function has no
other effect, the statement changes nothing.

A result written to the blank identifier is discarded the same way, so it is
reported too:

```go
_ = strings.ToUpper(name) // result of strings.ToUpper is discarded; the call has no other effect
```

The body of a function in the package under analysis is checked for effects.
A write to a value the function allocated is free. A write anywhere else, a map
write, a send, a receive, a select, a panic, and a go statement are effects. A
call inside the body must itself be free of effects.

A function from another package is not checked. The rule recognizes a fixed
list of standard library functions whose effect is their result, including
`strings.ToUpper`, `fmt.Sprintf`, `errors.New`, and the `time.Time` methods.
The list is the one staticcheck uses for SA4017.

Not reported:

- a call whose result is read, as by an assignment, a return, an argument, or a
  condition;
- a function with no results;
- a function with a parameter or receiver that can hold a reference, such as a
  pointer, slice, map, channel, interface, or function, since a write through
  the reference is not visible in the body;
- a recursive function, directly or through another function, since its purity
  is left unproven;
- a call through a function literal, a function value, or an interface method;
- a function from another package that is not on the standard library list;
- a deferred or go call, since only a call instruction is reported;
- a call in a test file, or in a function with a `*testing.B` parameter, since
  a test or benchmark may discard a result to exercise the call;
- a call in a branch that a constant condition makes unreachable.

Report only: the caller may have meant to assign the result, so there is no
fix. Removing the call and assigning the result are both plausible repairs.

The rule is adapted from staticcheck's SA4017
(https://staticcheck.dev/docs/checks/#SA4017). SA4017 uses whole-program purity
facts. This rule checks the body in the package under analysis and uses a fixed
list for the standard library.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/discardedpure/cmd/discardedpure
go tool discardedpure ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -discardedpure ./...
```
