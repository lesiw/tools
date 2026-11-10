# lesiw.io/tools/analyzers/nilreturn

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilreturn.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilreturn)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a return of the variable an if just found to be nil.

```go
func f(err error) error {
	if err == nil {
		return err // err is nil here, so the return is nil
	}
	return nil
}
```

The if tests whether the value is nil. Its body returns that same value, so the
return is always nil. Either the condition is inverted, or the return should be
nil.

The check reports only an error. A nil-receiver guard returns the receiver, and
that is the usual way to write a method that does nothing for a nil receiver.

The idea comes from go-critic's nilValReturn.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilreturn/cmd/nilreturn
go tool nilreturn ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilreturn ./...
```
