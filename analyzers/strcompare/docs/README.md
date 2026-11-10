# lesiw.io/tools/analyzers/strcompare

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/strcompare.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/strcompare)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison made through strings.Compare or bytes.Compare.

```go
func f(a, b string) {
	if strings.Compare(a, b) == 0 { // comparison through Compare
	}
	if strings.Compare(a, b) < 0 { // comparison through Compare
	}
	if strings.Compare(a, b) > 0 { // comparison through Compare
	}
	if strings.Compare(a, b) == -1 { // comparison through Compare
	}
	if 0 != strings.Compare(a, b) { // comparison through Compare
	}
}
```

strings.Compare exists for sorting and for three-way comparisons. Against zero,
the operator does the same with less code.

```go
a == b
```

The idea comes from go-critic's stringsCompare.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/strcompare/cmd/strcompare
go tool strcompare ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -strcompare ./...
```
