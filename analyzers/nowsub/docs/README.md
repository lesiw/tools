# lesiw.io/tools/analyzers/nowsub

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nowsub.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nowsub)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports `time.Now().Sub(t)` or `t.Sub(time.Now())`.

```go
func elapsed(start time.Time) time.Duration {
    return time.Now().Sub(start) // call can be replaced by time.Since(start)
}

func remaining(deadline time.Time) time.Duration {
    return deadline.Sub(time.Now()) // call can be replaced by time.Until(deadline)
}
```

The time package states the shorthand for each direction: "Since returns the
time elapsed since t. It is shorthand for time.Now().Sub(t)" and "Until
returns the duration until t. It is shorthand for t.Sub(time.Now())".

In the `Since` direction the receiver must be `time.Now()` itself, so
`time.Now().Add(d).Sub(start)` is left alone. `Since` reads the clock after
its argument, where the original reads it before, so the argument must do no
work: a conversion, a type assertion, and a builtin that only reads its
arguments are free, and any other call or receive leaves the call alone. The
builtins are `cap`, `complex`, `imag`, `len`, `max`, `min`, and `real`.

The receiver is examined first, so a call is tried in one direction only;
`time.Now().Sub(time.Now())` is left alone.

In the `Until` direction the receiver must be a `time.Time` or a `*time.Time`.
A struct that embeds a `time.Time` promotes `Sub`, and a call on one is left
alone, because the rewrite would have to name the embedded field.

A call is reported only when `Now` is written as a package selector, so
`Now()` from a dot import and `now()` from a variable holding `time.Now` are
left alone.

The rule is adapted from staticcheck's `S1012` and `S1024`, which do not check
the `Since` argument.

Fixable: the call becomes `time.Since(x)` or `time.Until(x)`, written with the
name the file gives the time package. A receiver of type `*time.Time` is
written as the value it points at, so `p` becomes `*p` and `&start` becomes
`start`. The fix is withheld when the call spans lines or holds a comment. A
rewrite that prints on more than one line is reported as a call to the
function, without its argument, and takes no fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nowsub/cmd/nowsub
go tool nowsub ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nowsub ./...
go tool lesiw.io/tools/cmd/fix -nowsub ./...
```
