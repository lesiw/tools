# lesiw.io/tools/analyzers/packagename

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/packagename.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/packagename)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a package name that gives no sense of its
contents, such as `util` or `misc`, a name with an underscore, a name that is
not all lower case, or a package named `pkg` whose directory has another base
name.

Adapted from revive's `package-naming`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/packagename/cmd/packagename
go tool packagename ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -packagename ./...
```
