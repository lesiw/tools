# lesiw.io/tools/analyzers/dupbranch

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dupbranch.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dupbranch)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if whose two branches hold the same statements.

```go
func f(cond bool) {
	if cond {
		println("same")
	} else { // both branches hold the same statements
		println("same")
	}
}
```

Both branches contain the same statements. The condition decides nothing.
Either it is left over from an edit, or one branch was meant to do something
else.

The idea comes from go-critic's dupBranchBody.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dupbranch/cmd/dupbranch
go tool dupbranch ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dupbranch ./...
```
