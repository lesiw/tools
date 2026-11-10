# lesiw.io/tools/analyzers/initialisms

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/initialisms.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/initialisms)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an initialism written in mixed case.

```go
package a

type URLPath struct{}

type urlPath struct{}

type UrlPath struct{} // Url should be URL in UrlPath

func f(httpURL string) {}

func g(httpUrl string) { // Url should be URL in httpUrl
	_ = httpUrl
}
```

Go writes an initialism in one case throughout a name: URLPath, urlPath or
urlpath, never UrlPath. A name that mixes the cases reads as two words.

The idea comes from golint's commonInitialisms.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/initialisms/cmd/initialisms
go tool initialisms ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -initialisms ./...
```
