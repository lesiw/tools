# lesiw.io/tools/analyzers/nilsliceappend

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilsliceappend.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilsliceappend)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an append to a nil slice where a slice
literal gives the result.

```go
func pair() []int {
    // append to a nil slice; write the slice literal
    return append([]int(nil), 1, 2)
}
```

The section of the spec on appending to and copying slices says append
"appends zero or more values x to a slice s of type S and returns the resulting
slice, also of type S" ([Appending to and copying
slices](https://go.dev/ref/spec#Appending_and_copying_slices)). With a nil s the
result holds only the appended values, which a slice literal gives directly.

```go
return []int{1, 2}
```

The fix replaces the call with the slice literal and keeps the type of the first
argument, so a nil slice of a named type or a type parameter with one underlying
slice type gives a literal of that type. In a control clause the literal of a
named type takes parentheses, where its brace would read as the block. The
literal's capacity is its length, where append may allocate more. A call with a
comment inside it is reported without a fix, since the edit would delete the
comment.

Only a direct conversion of nil is reported; a nil slice from a call, a slice
expression, or a variable is left alone. A base written as make([]T, 0) or []T{}
is not nil, so the call already starts from a fresh slice and is left alone. A
spread append is left alone, since an empty source can make the result nil and
no slice literal is nil. An append with no value after the nil slice is left
alone for the same reason.

The rule is adapted from staticcheck's S1019, which simplifies a make call by
omitting redundant arguments.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilsliceappend/cmd/nilsliceappend
go tool nilsliceappend ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilsliceappend ./...
go tool lesiw.io/tools/cmd/fix -nilsliceappend ./...
```
