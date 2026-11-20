# lesiw.io/tools/analyzers/recursiveiter

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/recursiveiter.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/recursiveiter)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an iterator that ranges over a call to
itself.

```go
func (l *list) All() iter.Seq[int] {
    return func(yield func(int) bool) {
        for v := range l.next.All() { // iterator All ranges over a call to itself
            if !yield(v) {
                return
            }
        }
    }
}
```

An iterator written this way ranges over the child's iterator at every level.
Each nested range-over-func typically heap-allocates a variable, and each
element passes through one dynamic yield call per enclosing range. A list of N
nodes costs O(N²) yield calls, and a balanced tree of N costs O(N log N).

A method that takes a predicate and walks the nodes makes one dynamic call per
element and allocates nothing:

```go
func (l *list) every(f func(int) bool) bool {
    return l == nil || f(l.value) && l.next.every(f)
}

func (l *list) All() iter.Seq[int] {
    return func(yield func(int) bool) {
        _ = l.every(yield)
    }
}
```

The range must sit inside a function literal that the function returns, and the
call must resolve to the declared function itself: a wrapper that ranges over
another type's iterator of the same name is left alone, and so is a call
through a function value. The only result must be `iter.Seq`, `iter.Seq2`, or
an alias of either. The unnamed `func(func(int) bool)` type and a defined type
whose underlying type is `iter.Seq` are left alone.

Vendored from gopls' `recursiveiter`
(`gopls/internal/analysis/recursiveiter`, `golang.org/x/tools/gopls` v0.24.0),
which is internal and cannot be imported. gopls reports a result whose type is
`iter.Seq` or `iter.Seq2` itself; an alias of either is reported here too.

Report only: the fix would add a traversal function to the recursive type, and
the tool cannot name it.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/recursiveiter/cmd/recursiveiter
go tool recursiveiter ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -recursiveiter ./...
```
