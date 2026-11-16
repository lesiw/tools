# lesiw.io/tools/analyzers/addrderef

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/addrderef.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/addrderef)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports the address of a pointer indirection.

```go
func f(p *int) *int {
    return &*p // &*x panics on a nil x; x does not
}
```

The [address operators](https://go.dev/ref/spec#Address_operators) section of
the spec says `*x` denotes the variable `x` points to, so `&*x` is `x` when `x`
and `&*x` have the same type. The rule for that comparison is in the
[type identity](https://go.dev/ref/spec#Type_identity) section. The address
operators section also says `&*x` raises a run-time panic when `x` is nil,
while `x` evaluates to nil. The fix removes both operators only where the
operand cannot be nil: an address such as `&v`, or a call to the builtin
`new`. Elsewhere the diagnostic carries no fix.

The fix is withheld when the edit would remove a comment.

A generated file, including cgo output, is left alone, since an edit would
change generated text instead of its source.

An address passed to `unsafe.Pointer`, possibly through a conversion, is left
alone, since cgo uses that form to check only the pointed-to object.

A defined pointer type is left alone. `&*x` has the unnamed pointer type, and
the two expressions have different types. A type parameter whose core type is
a pointer is left alone for the same reason.

The reverse form, `*&x`, is a dereference of an address, so it is outside this
rule.

The rule is like staticcheck's SA4001.

Fixable: `&*&v` becomes `&v` and `&*new(T)` becomes `new(T)`. An inner `&*`
gets no fix while an enclosing `&*` has one, so the fixes do not overlap.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/addrderef/cmd/addrderef
go tool addrderef ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -addrderef ./...
go tool lesiw.io/tools/cmd/fix -addrderef ./...
```
