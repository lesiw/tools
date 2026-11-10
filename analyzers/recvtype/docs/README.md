# lesiw.io/tools/analyzers/recvtype

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/recvtype.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/recvtype)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a type whose methods mix pointer and value receivers.

```go
func (m mixed) two() {} // methods of mixed mix pointer and value receivers

type pointerOnly struct{}

func (p *pointerOnly) one() {}

func (p *pointerOnly) two() {}

type valueOnly struct{}

func (v valueOnly) one() {}

func (v valueOnly) two() {}

type many struct{}

func (m *many) one() {}

func (m many) two() {} // methods of many mix pointer and value receivers

func (m *many) three() {}
```

A value receiver works on a copy. A method that changes the value through one
changes nothing the caller can see. One kind for the whole type also settles
which methods a value has.

The idea comes from the recvcheck linter.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/recvtype/cmd/recvtype
go tool recvtype ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -recvtype ./...
```
