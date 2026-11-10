# lesiw.io/tools/analyzers/recvname

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/recvname.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/recvname)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a receiver name that differs from its type's other methods.

```go
func (cl *client) post() {} // receiver name "cl" differs from "c"

type other struct{}

func (o other) one() {}

func (o *other) two() {}

func (this *other) three() {} // receiver "this"; use the type's initial

type quiet struct{}

func (quiet) zero() {}

func (q *quiet) one() {}

type box[T any] struct{}

func (b *box[T]) get() {}

func (b *box[T]) put() {}

// The odd name out is the one reported, whichever order the file is in.
type bulk struct{}

func (z *bulk) a() {}

func (z *bulk) b() {}

func (z *bulk) c() {}

func (x *bulk) d() {} // receiver name "x" differs from "z"
```

One type gets one receiver name. A reader who sees two names for the same value
has to check whether the second is a different value.

The idea comes from revive's receiver-naming.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/recvname/cmd/recvname
go tool recvname ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -recvname ./...
```
