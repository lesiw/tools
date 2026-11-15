# lesiw.io/tools/analyzers/nowsub

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nowsub.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nowsub)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports `time.Now().Sub(t)` and `t.Sub(time.Now())`.

```go
func elapsed(start time.Time) time.Duration {
    return time.Now().Sub(start) // call can be replaced by time.Since(start)
}

func remaining(deadline time.Time) time.Duration {
    return deadline.Sub(time.Now()) // call can be replaced by time.Until(deadline)
}
```

The time package states the shorthand for each direction: "Since returns the
time elapsed since t. It is shorthand for time.Now().Sub(t)" and "Until returns
the duration until t. It is shorthand for t.Sub(time.Now())".

In `time.Now().Sub(x)` the receiver must be `time.Now()` itself, so
`time.Now().Add(d).Sub(t)` is left alone. In `x.Sub(time.Now())` the receiver
must be a `time.Time` or a `*time.Time`; a struct that embeds `time.Time`
promotes `Sub`, and that call is left alone. A call is examined only when `Now`
is package-qualified, so a file that dot-imports time is not reported.

The rule is adapted from staticcheck's `S1012` and `S1024`, which report the
two directions separately.

Fixable: the call becomes `time.Since(x)` or `time.Until(x)`, written with the
name the file gives the time package, so a renamed import keeps its name. A
receiver of type `*time.Time` is written as the value it points at, so `p`
becomes `*p` and `&start` becomes `start`. The fix is withheld when the replaced
call spans lines or holds a comment. In the `Since` direction a call is reported
only when the argument does no work, because the rewrite moves the clock read
past the argument.

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
