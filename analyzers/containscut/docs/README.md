# lesiw.io/tools/analyzers/containscut

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/containscut.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/containscut)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `Contains` check whose answer `Cut`
already returns.

```go
if strings.Contains(ref, "@") { // Contains followed by Cut; Cut reports the separator itself
	repo, _, _ := strings.Cut(ref, "@")
	return repo, "", true
}
```

`Cut` reports whether the separator was present, so the `Contains` is a second
scan for a boolean the next line throws away. The idiomatic form asks once:

```go
if repo, _, ok := strings.Cut(ref, "@"); ok {
	return repo, "", true
}
```

Report only. The rewrite moves a declaration into the condition, which is the
author's layout decision.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/containscut/cmd/containscut
go tool containscut ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -containscut ./...
go tool lesiw.io/tools/cmd/fix -containscut ./...
```
