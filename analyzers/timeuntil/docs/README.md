# lesiw.io/tools/analyzers/timeuntil

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/timeuntil.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/timeuntil)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of `time.Now().Add(d)` with a
`time.Time`. The same test is a comparison of `time.Until(t)` or
`time.Since(t)` with `d`.

```go
func f(deadline time.Time, wait time.Duration) bool {
    return time.Now().Add(wait).Before(deadline) // comparison can be replaced by time.Until(deadline) > wait
}

func g(deadline time.Time) bool {
    return deadline.After(time.Now().Add(-time.Minute)) // comparison can be replaced by time.Since(deadline) < time.Minute
}
```

The time package documentation gives the shorthand for each direction: "Until
returns the duration until t. It is shorthand for t.Sub(time.Now())" and
"Since returns the time elapsed since t. It is shorthand for
time.Now().Sub(t)".
`time.Now().Add(d).Before(t)` is `time.Until(t) > d`.

The rule is adapted from staticcheck's `S1024`, which reports
`t.Sub(time.Now())` alone. This rule reports the same test written with
`Before` and `After`; `nowsub` reports `t.Sub(time.Now())` and
`time.Now().Sub(t)`.

## Checks

### Before and After

`Before` and `After` compare two `time.Time` values. With
`time.Now().Add(d)` on either side, the same test is a comparison of a duration
with `d`:

- `time.Now().Add(d).Before(t)` and `t.After(time.Now().Add(d))` become
  `time.Until(t) > d`.
- `time.Now().Add(d).After(t)` and `t.Before(time.Now().Add(d))` become
  `time.Until(t) < d`.

A receiver of type `*time.Time` is written as the value it points at, so
`p.After(...)` becomes `time.Until(*p) > d` and `(&t).Before(...)` becomes
`time.Until(t) < d`.

### A negated duration

An `Add` argument of the form `-d` is written with `Since` when `d` and `-d`
both fit in a `time.Duration`, and the minus is dropped. For example,
`t.After(time.Now().Add(-time.Minute))` becomes
`time.Since(t) < time.Minute`. A variable is written with `Until`, with the
minus left in place, because a variable may be `MinInt64`:
`t.After(time.Now().Add(-d))` becomes `time.Until(t) > -d`. A negative value
written another way, as in `Add(-1 * d)`, is written with `Until` and the
minus is kept.

### A negated comparison

The `!` is rewritten along with the comparison. An odd number of them changes
the operator from strict to non-strict; an even number leaves the operator
unchanged. `!time.Now().Add(d).Before(t)` becomes `time.Until(t) <= d`.

## Not reported

- A `time.Now` call bound to a variable, as in `now := time.Now; now().Add(d)`.
- `Sub` and `Equal`. The rule matches `Before` and `After` only; a shifted
  `Sub`, as in `time.Now().Add(d).Sub(t)`, is left alone.
- `AddDate`, which shifts by calendar fields. The rule matches only `Add` with
  a duration.
- A chained `Add`, as in `time.Now().Add(a).Add(b)`. The rule matches only a
  direct `time.Now().Add(d)`.
- A struct that embeds `time.Time`, even though its method set promotes
  `Before` and `After`.
- A `Before` or `After` method on another type, as in a type with the same
  method names.
- A comparison whose time operand or duration has an effect, when
  `time.Now().Add(d)` is the receiver: a call other than a conversion or a free
  builtin, a channel receive, or the address of a composite literal. The
  rewrite evaluates the time operand before the clock read and before the
  duration in that position; with the call as the argument, the calls keep
  their order. A builtin that only reads its arguments is free.
- An addressable time operand (a variable, field, index, or dereference), a
  conversion or type assertion, or a pointer, compared with an effectful
  duration when `time.Now().Add(d)` is the argument. The receiver's value may
  be read after the arguments, and the rewrite reads it before the duration.
- A `Now` call reached through a dot import, with no package selector. The rule
  matches only a qualified `Now` call.
- A call used as an expression statement, or as the call of a `go` or `defer`
  statement. The call's result is discarded, and a comparison cannot be used
  there.
- A method expression or a method value, as in
  `time.Time.Before(deadline, ...)` or `before := deadline.Before`. The rule
  matches a direct method call.

## Fixes

The comparison becomes a duration comparison with `Until` or `Since`, written
with the name the file uses for the time package. The fix is withheld when the
comparison spans lines or contains a comment. A rewrite that prints on more
than one line is reported as a comparison with the helper, and no fix is
offered.
When the comparison is an operand of `==` or `!=`, the rewrite is parenthesized
so the grouping is unchanged.

The rewrite compares durations, where the original compares instants.
`time.Until` and `time.Since` saturate at the int64 limits, so a span beyond
about 292 years can have a different answer.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/timeuntil/cmd/timeuntil
go tool timeuntil ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -timeuntil ./...
go tool lesiw.io/tools/cmd/fix -timeuntil ./...
```
