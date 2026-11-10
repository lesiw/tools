# lesiw.io/tools/analyzers/zerorepeat

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/zerorepeat.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/zerorepeat)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a run of zero bytes built with bytes.Repeat.

```go
func f(n int) []byte {
	return bytes.Repeat([]byte{0}, n) // zero bytes from bytes.Repeat
}
```

bytes.Repeat allocates the pattern and then the result. make allocates the
result and leaves it zeroed, which is the value the call was building.

```go
make([]byte, n)
```

The idea comes from go-critic's zeroByteRepeat.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/zerorepeat/cmd/zerorepeat
go tool zerorepeat ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -zerorepeat ./...
```
