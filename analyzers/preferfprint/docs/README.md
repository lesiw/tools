# lesiw.io/tools/analyzers/preferfprint

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/preferfprint.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/preferfprint)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a formatted string written to a writer.

```go
func f(b *bytes.Buffer, w *bytes.Buffer, n int) {
	b.Write([]byte(fmt.Sprintf("%d", n))) // formatted string written to a writer
	b.WriteString(fmt.Sprintf("%d", n)) // formatted string written to a writer
	w.WriteString(fmt.Sprint(n)) // formatted string written to a writer
	io.WriteString(b, fmt.Sprintln(n)) // formatted string written to a writer
}
```

Building the string and then writing it allocates the string in between.
Fprintf formats straight into the writer.

```go
fmt.Fprintf(w, "%d", n)
```

The idea comes from go-critic's preferFprint.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/preferfprint/cmd/preferfprint
go tool preferfprint ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -preferfprint ./...
go tool lesiw.io/tools/cmd/fix -preferfprint ./...
```
