# lesiw.io/tools/analyzers/elseif

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/elseif.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/elseif)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if nested in an else block.

```go
func f(n int) int {
	if n == 0 {
		return 0
	} else { // if nested in an else; write else if
		if n == 1 {
			return 1
		}
	}
	return -1
}
```

The nested if is the only statement in the else. The braces and the extra
indent add nothing.

```go
} else if n == 1 {
```

The idea comes from go-critic's elseif.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/elseif/cmd/elseif
go tool elseif ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -elseif ./...
go tool lesiw.io/tools/cmd/fix -elseif ./...
```
