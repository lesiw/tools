# lesiw.io/tools/analyzers/importshadow

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/importshadow.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/importshadow)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a declaration that shadows an imported package name.

```go
func f(path string) string { // declaration shadows the imported package "path"
	return path
}
```

A package name is a qualifier used throughout the file. A local with the same
name changes the meaning of every later reference. A reader who misses the
local reads it as the package.

The idea comes from go-critic's importShadow.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/importshadow/cmd/importshadow
go tool importshadow ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -importshadow ./...
```
