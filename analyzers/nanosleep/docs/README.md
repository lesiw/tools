# lesiw.io/tools/analyzers/nanosleep

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nanosleep.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nanosleep)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a bare small integer passed to
`time.Sleep`.

```go
func pause() {
    time.Sleep(1) // time.Sleep(1) reads the argument as 1 nanosecond; write 1 * time.Nanosecond
}
```

The time package documents `Duration` as "an `int64` nanosecond count" and
provides constants such as `time.Second`, `time.Millisecond`, and
`time.Nanosecond`. A bare integer is therefore read as nanoseconds, so
`time.Sleep(1)` sleeps for one nanosecond.

The rule reports integer literals from 1 through 120 inclusive. A zero argument
does nothing, and a number above 120 may be deliberate; neither is reported.
Only a literal is reported, so a named constant, a conversion such as
`time.Duration(1)`, and an expression such as `n * time.Nanosecond` are left
alone. A parenthesized literal is still a literal. The call must resolve to the
package-level `time.Sleep`, through any import name; a method named `Sleep`, a
function value, and a function declared in the package are left alone.

Fixable: `time.Sleep(1)` becomes `time.Sleep(1 * time.Nanosecond)`, using the
import name in that file, so an import named `t` yields `1 * t.Nanosecond`. The
duration is unchanged. It is withheld for a dot import, where there is no import
name to qualify `Nanosecond` with, or when the rewrite would take the line past
the width; the call is still reported in both cases.

The rule is inspired by staticcheck's `SA1004`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nanosleep/cmd/nanosleep
go tool nanosleep ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nanosleep ./...
go tool lesiw.io/tools/cmd/fix -nanosleep ./...
```
