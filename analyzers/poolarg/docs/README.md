# lesiw.io/tools/analyzers/poolarg

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/poolarg.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/poolarg)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a value stored in a `sync.Pool` that is not
one pointer word.

```go
func keep(p *sync.Pool, b []byte) {
    p.Put(b) // value stored in the pool is not a pointer; store a pointer
}
```

`sync.Pool`'s purpose is "to cache allocated but unused items for later reuse,
relieving pressure on the garbage collector". The compiler stores a value
directly in an interface's data word only when the value is one word that a
collector reads as a pointer (`IsDirectIface` in `cmd/compile/internal/types`:
"Can this type be stored directly in an interface word?"). For any other value
whose type is not an interface, the data word points at memory holding it: a
copy of a slice header, a string, or a struct, or static data the compiler
already holds.

A call on a pool, on an embedded pool, or through a method expression such as
`(*sync.Pool).Put(p, b)` is reported at the value it stores, and a `Put` held
in a variable, as in `put := p.Put; put(b)`, is not reported at all. A slice is
reported even when its element type is a type parameter, since a slice header
has one representation.

The rule tests the type of the argument; the only syntax it reads is whether
go/types folds the argument to a constant. A non-constant integer from 0
to 255 and a zero-value struct literal also reach the pool without an
allocation, and the rule reports both.

Not reported:

- a pointer, a map, a chan, a func, or an `unsafe.Pointer`, which is one word
  that a collector reads as a pointer;
- a struct or array of one such word, unless the compiler adds padding after
  that word;
- an argument whose type is an interface, which the pool holds as two words;
- `nil`: `Put` returns before storing it, and go/types gives an untyped nil no
  size;
- a value of a type parameter, and a struct or array that holds one;
- a value that occupies no bytes, such as `struct{}` or `[0]byte`, which has
  nothing to copy;
- a constant expression, which the compiler stores once in read-only data.

The rule is adapted from staticcheck's SA6002, which reports a value whose type
is not pointer-like, and a slice even though it is. This rule compares the size
of the type, so it reports fewer values than SA6002.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/poolarg/cmd/poolarg
go tool poolarg ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -poolarg ./...
```
