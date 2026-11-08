# lesiw.io/tools/analyzers/testhelpers

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/testhelpers.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/testhelpers)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that keeps tests on the `testing.T` native
helpers.

The `testing` package already provides context, environment,
temporary-directory, and working-directory plumbing scoped to the
test's lifetime; the process-global equivalents leak state across
tests.

## Checks

Inside a function with a named `*testing.T` or `*testing.B`
parameter:

```go
ctx := context.Background() // context.Background() can be replaced by t.Context()
os.Setenv("K", "V")         // os.Setenv can be replaced by t.Setenv
```

`context.TODO` reports and fixes the same way as
`context.Background`. The `os.Setenv` fix applies to a bare call
statement; a checked error needs its handling removed by hand, so
that form is reported without a fix. A fix can leave the `context`
or `os` import unused; `goimports` cleans it up. `os.MkdirTemp` and `os.Chdir`
report toward `t.TempDir` and `t.Chdir` with no fix; their
return-value plumbing has no single mechanical rewrite.

A function literal without its own `*testing.T` or `*testing.B` —
a `t.Cleanup` body, a goroutine — is not examined: the test's
context is already ending there. Only package-qualified calls are
matched, and generated files are skipped.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/testhelpers/cmd/testhelpers
go tool testhelpers ./...
```
