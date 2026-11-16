# lesiw.io/tools/analyzers/timeequal2

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/timeequal2.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/timeequal2)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of a `time.Time` with the
zero-value literal.

```go
func unset(t time.Time) bool {
    return t == time.Time{} // time.Time compared with the zero value; write t.IsZero()
}
```

The `Time` doc says: "The zero value of type Time is January 1, year 1,
00:00:00.000000000 UTC. As this time is unlikely to come up in practice, the
`Time.IsZero` method gives a simple way of detecting a time that has not been
initialized explicitly." `IsZero` tests the instant alone. The `==` operator
also compares the location and the monotonic clock reading, so a zero instant
in a named location can compare unequal to `time.Time{}` while `IsZero`
returns true. A comparison with the literal is usually a test for an unset
time, and `IsZero` is that test.

Only a comparison with an empty composite literal of type `time.Time` is
reported, including one written through an alias. A comparison with a variable
containing the zero value is not reported, because the analyzer does not track
values. A comparison between two zero literals is not reported because neither
operand is the time under test. A comparison used as an operand of another
comparison is reported without a fix, because the outer comparison uses the
result as a value. That shape is often a deliberate test of the operator.

A defined type whose underlying type is that of `time.Time` is not reported,
even if it declares `IsZero`.

A `case` in a switch on a `time.Time` and a map keyed by one are not
comparisons, so they are not reported.

Fixable: the comparison becomes `IsZero`, or its negation for `!=`. A `!`
outside the comparison is folded into the replacement, so
`!(t != time.Time{})` becomes `t.IsZero()`. The fix is withheld when the
comparison has a defined boolean type, when a comment sits inside the text it
would replace, when the comparison spans more than one line, and when the
rewrite would take the line past the width. A line already past the width still
takes the fix.

The rule is like revive's time-equal, which reports a `time.Time` compared with
`==` or `!=` without a fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/timeequal2/cmd/timeequal2
go tool timeequal2 ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -timeequal2 ./...
go tool lesiw.io/tools/cmd/fix -timeequal2 ./...
```
