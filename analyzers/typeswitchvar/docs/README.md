# lesiw.io/tools/analyzers/typeswitchvar

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/typeswitchvar.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/typeswitchvar)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a type switch whose cases assert the value again.

```go
func f(x any) int {
	switch x.(type) { // type switch binds nothing
	case int:
		return x.(int)
	case string:
		return len(x.(string))
	}
	return 0
}
```

The switch tests the type. Each case asserts the value again. Binding the value
in the header does both once.

```go
switch v := x.(type) {
case int:
	return v
}
```

The idea comes from go-critic's typeSwitchVar.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/typeswitchvar/cmd/typeswitchvar
go tool typeswitchvar ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -typeswitchvar ./...
```
