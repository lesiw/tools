# lesiw.io/tools/analyzers/nilmapassign

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilmapassign.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilmapassign)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an assignment to an element of a nil map.

```go
func count(words []string) map[string]int {
    var counts map[string]int
    for _, word := range words {
        counts[word]++ // assignment to nil map
    }
    return counts
}
```

The specification's "Index expressions" section says that assigning to an
element of a nil map causes a run-time panic. "Map types" says the value of an
uninitialized map is nil, so a local map variable stays nil until a make call,
a map literal, or another value reaches it.

A write is reported when the map is nil on every path to it. The rule follows
the value through copies and merges, so a nil variable that another variable
copies is still reported, and a variable assigned a map on one branch is not.
An assignment, a compound assignment such as `m[k] += v`, and `m[k]++` are all
reported.

A map that arrives from a caller is not this rule's subject: a parameter, a
receiver, a result, a field, an element of a collection, or a call may hold a
map another function made. Package-level variables are left alone for the same
reason. A map captured by a function literal is left alone too, since the
literal can run after the enclosing function gives the map a value. A map whose
address is taken is also left alone, since a write through the pointer can set
it first.

Report only: the map has to be made before the write, and the analyzer cannot
choose where. A literal at the declaration, a make call before a loop, and a
field set by the caller are all plausible repairs.

The rule is adapted from staticcheck's SA5000.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilmapassign/cmd/nilmapassign
go tool nilmapassign ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilmapassign ./...
```
