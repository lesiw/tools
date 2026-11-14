# lesiw.io/tools/analyzers/addrnil

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/addrnil.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/addrnil)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of an address with nil that is
always the same.

```go
func unset(p *int) bool {
	return &p == nil // an address cannot be nil
}
```

Taking an address never yields nil, so the comparison has the same answer
whenever it is evaluated. When the operand is a pointer variable, the nil check
is `p == nil`: the `&` is the mistake.

The nil operand has to be the identifier `nil` itself: `&x == (*int)(nil)` and a
comparison against a variable named `nil` are left alone.

The rule is like staticcheck's SA4022.

Report only. Writing the answer in place drops the evaluation of the operand,
which panics on a nil `&*p` or an out-of-range `&xs[i]`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/addrnil/cmd/addrnil
go tool addrnil ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -addrnil ./...
```
