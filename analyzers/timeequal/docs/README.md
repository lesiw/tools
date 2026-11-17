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

func unset(t time.Time) bool {
    return t == time.Time{} // time.Time compared with the zero value; write t.IsZero()
}
```

The time package says what the operator does: "Note that the Go == operator
compares not just the time instant but also the Location and the monotonic
clock reading." The `Time` doc adds: "In general, prefer t.Equal(u) to t == u,
since t.Equal uses the most accurate comparison available and correctly
handles the case when only one of its arguments has a monotonic clock
reading." It says that "the Time.IsZero method gives a simple way of detecting
a time that has not been initialized explicitly", which is what a comparison
against the zero-value literal tests for. `IsZero` tests the instant alone, so
a zero instant in a named location compares unequal to `time.Time{}` though
`IsZero` returns true.

## Checks

Both operands must be a `time.Time`, and the comparison must be written with
the operator.

Not reported:

- a `case` in a switch on a time and a map keyed by one; neither is a
  comparison;
- a comparison of two zero-value literals, because neither operand is the time
  under test;
- a comparison of a struct that holds a time, because the operands are not
  times;
- a comparison of a defined type whose underlying type is `time.Time`, even
  when the type declares `IsZero`.

A test that checks a time's representation writes the comparison deliberately,
and the rule reports it anyway.

## Fixes

The fix writes `Equal` for a comparison of two `time.Time` values, and `IsZero`
for a comparison against the zero-value literal. Either form is negated for
`!=`, and a `!` outside the comparison is folded into the replacement, so
`!(t != time.Time{})` becomes `t.IsZero()`. The receiver is parenthesized where
a selector would bind to its own operand, as in `(*p).IsZero()` and
`(<-ch).IsZero()`, and where it begins with a composite literal, since a
statement header cannot begin with one.

The suggestion in the message, as in `; write t.IsZero()`, is left out when the
receiver or the argument prints on more than one line, when the comparison has
a defined boolean type, or when another comparison takes the result of a
zero-value comparison as an operand. That last shape is usually a deliberate
test of what the operator compares, so it takes no fix either. The fix is
withheld when a comment sits inside the text it would replace, when the
comparison spans lines, and when the rewrite would take the line past the
width. A line already past the width still takes the fix.

The idea comes from revive's time-equal, which reports the comparison without a
fix.

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
