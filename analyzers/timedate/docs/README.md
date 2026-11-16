# lesiw.io/tools/analyzers/timedate

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/timedate.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/timedate)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a numeric date written in a format that
does not sort.

```go
const date = "3/1/2024" // text does not sort as a date; the layout 2006-01-02 does
```

A year-first date sorts the way the dates do, and `3/1/2024` does not: it reads
as March 1 in one country and January 3 in another, and its text sorts by month.

The analyzer reads a string literal whose whole text is a numeric date in a
form that does not sort: the year last, or a year-first date with an unpadded
field, as `2024-3-1`. A literal whose text sorts is left alone.

A 24-hour clock may follow the date after a space or a `T`, with a fractional
second or without one. The fraction is read after a dot or a comma, when it is
all digits and at most nine of them; a fraction that is not read leaves the
literal unreported. The hour may be a single digit, as `3/1/2024 3:04`, which is
reported, and the layout the analyzer names pads it. A clock whose minute or
second is a single digit, one that carries an `AM` or `PM` mark, and one with a
zone after it, are not read, and the date in front of one is not reported.

A form that names the month is left alone, because the analyzer reads numeric
fields only. A form with a dot between its fields is left
alone, since three dot-separated numbers are as often a version as a date, and
so is a form with no separator, whose fields cannot be told apart. A two-digit
year is left alone as well: a two-digit field can be a month, a day, or a year,
and nothing in the text says which.

A layout is not a date, and a literal in the layout or parsed-value argument of
a time call is skipped. Every other literal is read as a date, including one a
call takes from a variable and one a method value or an interface method
receives. A layout among those writes its year as 2006, so a literal that
parses into that year is left alone; one that names any other year is read as a
date.

Report only: rewriting the literal changes the text this program writes or
reads, and the format the other system expects is not visible here.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/timedate/cmd/timedate
go tool timedate ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -timedate ./...
```
