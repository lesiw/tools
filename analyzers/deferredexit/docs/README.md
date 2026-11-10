# lesiw.io/tools/analyzers/deferredexit

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deferredexit.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deferredexit)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an exit that happens while deferred calls are pending.

```go
func f(name string) {
	defer os.Remove(name)
	if name == "" {
		log.Fatal("empty name") // exit skips the deferred calls
	}
}
```

os.Exit and log.Fatal end the process without running deferred calls. An exit
after a defer skips it. An exit before the first defer is fine, because nothing
is pending.

The idea comes from go-critic's exitAfterDefer.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deferredexit/cmd/deferredexit
go tool deferredexit ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deferredexit ./...
```
