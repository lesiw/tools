# lesiw.io/tools/analyzers/mapkey

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/mapkey.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/mapkey)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a map key that is unhashable.

```go
func f(m map[any]int, xs []int) {
	_ = map[any]int{xs: 1} // map key of type []int is unhashable
	m[xs] = 1              // map key of type []int is unhashable
	delete(m, xs)          // map key of type []int is unhashable
}
```

A slice, a map, and a function are unhashable, and so is a struct or array that
holds one. The compiler rejects such a key type. An interface key type accepts
those values, and the map panics when it hashes one.

The rule reports the operand's type for a conversion to an interface, because
the operand is the value the map hashes, so the report for `any(xs)` names
`[]int`.

An interface-typed key is left alone, and so are an interface-typed struct field
or array element and a call returning an interface: the hash is the value's, and
the type does not name that value. The rule does not read a type set, so a key,
a map, a struct field, or an array element whose type is a type parameter is
left alone.

The name comes from go-critic's `mapKey`, which reports a string key with a
space at either end, or a repeated key.

Report only: several hashable forms of a key exist, and none is the right one.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/mapkey/cmd/mapkey
go tool mapkey ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -mapkey ./...
```
