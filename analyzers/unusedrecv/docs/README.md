# lesiw.io/tools/analyzers/unusedrecv

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unusedrecv.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unusedrecv)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a receiver that the method body never reads.

```go
func (t *T) Read() int { // receiver t is unused
	return 0
}
```

Go allows an unnamed receiver, and only a receiver. A name the body never reads
suggests the method works on the value. A reader has to check the body to learn
that it does not.

```go
func (*T) Read() int {
	return 0
}
```

The idea comes from revive's unused-receiver.

Fixable: the name is removed.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unusedrecv/cmd/unusedrecv
go tool unusedrecv ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unusedrecv ./...
go tool lesiw.io/tools/cmd/fix -unusedrecv ./...
```
