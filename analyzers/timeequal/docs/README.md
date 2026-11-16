# lesiw.io/tools/analyzers/timeequal

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/timeequal.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/timeequal)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `time.Time` compared with `==` or `!=`.

```go
func f(a, b time.Time) bool {
    return a == b // == on a time.Time compares more than the instant; write a.Equal(b)
}
```

The time package says what the operator does: "Note that the Go == operator
compares not just the time instant but also the Location and the monotonic
clock reading." The `Time` doc adds: "In general, prefer t.Equal(u) to t == u,
since t.Equal uses the most accurate comparison available and correctly
handles the case when only one of its arguments has a monotonic clock
reading."

A comparison against the zero-value literal is left to `timeequal2`, whose
fix writes `IsZero`.

The analyzer reports only a comparison written with the operator, between two
`time.Time` values that are not the zero literal. A `case` in a switch on one,
a map keyed by one, and a comparison of a struct or of a defined type that
contains one are not reported.

A comparison can be deliberate: a test that checks a time's representation is
written the same way as a mistake.

The idea comes from revive's time-equal, which reports the comparison without a
fix.

Fixable: the comparison becomes `Equal`. The fix is withheld when the
comparison has a defined boolean type, when it would take the line past the
width, when the comparison spans lines, and when a comment sits inside it. A
line already past the width still takes the fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/timeequal/cmd/timeequal
go tool timeequal ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -timeequal ./...
go tool lesiw.io/tools/cmd/fix -timeequal ./...
```
