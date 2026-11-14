# lesiw.io/tools/analyzers/timelayout

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/timelayout.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/timelayout)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a time layout that does not spell
the reference time: one that names no reference element, one that
prints the hour of a 12-hour clock with no `PM` or `pm`, or one whose
text does not parse back.

A layout is the reference time, Mon Jan 2 15:04:05 MST 2006, written
the way the output should read. The `time` package doc lists the names
each element can take, and the package echoes every other character
verbatim, so a wrong layout prints something plausible and reports no
error.

The layout is checked when it is a constant passed to `Format`,
`AppendFormat`, `Parse`, or `ParseInLocation`, including through a dot
import or a method expression. A layout in a variable is not checked,
and neither is a call through a function value, a method value, or an
interface method.

## Checks

### A layout names no reference element

```go
t.Format("yyyy-mm-dd") // layout names no reference element
t.Format("%Y-%m-%d")   // layout names no reference element
```

A date pattern from another system names no element of `time`, so the
output is the literal text. A fractional second that mixes `0` and `9`
is text too, and a three-letter month or weekday followed by a
lowercase letter is text: `Monitor` prints `Monitor`.

A digit that looks like an element can be text as well: `"2006-08-02"`
prints `08` where the month belongs, and nothing reports it.

One element is enough to pass, even beside text that is not an element:
`"2006-01-02 15:04:05 yyyy"` prints the date followed by `yyyy`.

### A 12-hour clock has no PM or pm

```go
t.Format("3:04") // layout prints the hour of a 12-hour clock with no PM or pm
```

The `time` doc pairs the two in its hour row, and gives the mark a row
of its own:

```
Hour: "15" "3" "03" (PM or AM)
AM/PM mark: "PM"
```

Without the mark nothing separates the morning 3:04 from the afternoon
3:04, and both print as `3:04`. Writing `15` prints the hour on a
24-hour clock, which needs no mark.

Every `3` is that hour, so a `3` meant as text prints the hour instead:
`+0300` prints `+0800` at 08:04, and `v0.3` prints `v0.8`. No layout can
print a literal `3`.

`AM` and `am` are not names of the mark, so `3:04 AM` has no mark and is
reported. Text is not what the check reads, so `15:04 AM` prints `AM` at
every hour and is not reported.

The check runs on the calls that print, `Format` and `AppendFormat`. A
parse layout has to match the text it is given, so it needs no mark:
`time.Parse("3:04", "8:09")` returns 8:09 AM, and a mark in the layout
would reject that input.

The analyzer reads one layout at a time, so `t.Format("3:04") + " " +
t.Format("PM")` prints the right hour and is still reported.

### A layout does not parse back the text it prints

```go
t.Format("2024-01-02")  // layout does not parse back the text it prints
t.Format("Jan 2, 2024") // layout does not parse back the text it prints
```

`2024` is not a year name. The `2` is a day, the `02` beside it
another, and the `4` a minute, so the minute has no digits left to
read.

The check reads one instant, so a layout that only fails to read back
at another instant is not reported: `"Jan 02 2006 MST"` reads back at
the reference time, and fails for a zone whose abbreviation is numeric.

A transposition passes as well: `"2006-02-01"` puts the day where the
month belongs, and the parse reads back what the format wrote.
`golang.org/x/tools`' `timeformat`, which the suite runs, reports that
one literal.

This check is adapted from staticcheck's SA1002, which parses a
layout's own text through `time.Parse` and covers `time.Parse` alone.

No fix is offered: a layout that names no element could become
anything, one that lacks its `PM` or `pm` has two repairs (`"3:04"`
becomes `"3:04 PM"` or `"15:04"`), and the digits here may be literal
text the author meant.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/timelayout/cmd/timelayout
go tool timelayout ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -timelayout ./...
```
