# lesiw.io/tools/analyzers/errflow

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errflow.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errflow)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an else block that follows a terminating statement.

```go
func returning(err error) int {
	if err != nil {
		return 0
	} else { // unnecessary else
		return 1
	}
}
```

The if block ends in a return. The else runs only when the condition is false,
and its code could run at the function's own level. The indent adds nothing.

The check leaves an else-if chain alone, because a chain lists conditions on
one value. It also leaves an if with an init statement alone, because the else
block can see a name declared there.

The idea comes from revive's indent-error-flow.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errflow/cmd/errflow
go tool errflow ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errflow ./...
```
