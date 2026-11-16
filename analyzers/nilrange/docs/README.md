# lesiw.io/tools/analyzers/nilrange

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilrange.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilrange)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a nil check around a range loop over a
slice or a map.

```go
func walk(xs []int) {
    if xs != nil { // nil check around range is unnecessary
        for range xs {
        }
    }
}
```

The section of the spec on for statements says that "if the map is nil, the
number of iterations is 0"
([For statements](https://go.dev/ref/spec#For_statements)), and the section on
length and capacity says the length of a nil slice is 0
([Length and capacity](https://go.dev/ref/spec#Length_and_capacity)). A range
over a nil slice or map therefore runs its body zero times, so the check is
redundant.

The check is an `if` with no init and no else, whose condition compares a
slice or map with nil using `!=`, and whose body is one range loop over that
value. The loop may be labeled. Either operand may be nil, and parentheses are
allowed. The value must be a name or a selector path; an index, a dereference,
and a call are left alone.

A slice, a map, a defined type with one of those underlying types, and a type
parameter whose core type is a slice or a map are reported. A channel is left
alone, since a range over a nil channel blocks forever. So is a pointer to an
array, since a range over a nil one ranges over the array's length and may
panic when it reads an element, and a function, since a range over a nil one
panics as well. A check that uses `==` is left alone, as are an `if` with an
init or an else, a body with another statement beside the loop, and a range
over a different value.

The fix replaces the `if` statement with the loop. The fix is withheld when a
comment outside the loop would be removed.

The rule is adapted from staticcheck's `S1031`.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilrange/cmd/nilrange
go tool nilrange ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilrange ./...
go tool lesiw.io/tools/cmd/fix -nilrange ./...
```
