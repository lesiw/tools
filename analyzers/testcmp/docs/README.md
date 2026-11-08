# lesiw.io/tools/analyzers/testcmp

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/testcmp.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/testcmp)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports uses of `reflect.DeepEqual` in
tests.

`DeepEqual` is not configurable: unexported fields are always
compared, a nil slice never equals an empty one, and `NaN` never
equals itself. A comparator such as
[go-cmp](https://github.com/google/go-cmp) lets a test say what
equal means and shows what differed.

## Checks

### reflect.DeepEqual in a test

```go
if !reflect.DeepEqual(a, b) { // avoid reflect.DeepEqual in tests; consider go-cmp
    t.Errorf("got %v, want %v", a, b)
}
```

Production (non-test) uses of `reflect.DeepEqual` are not
reported, and only package-qualified calls are matched — a dot
import or a function value is not.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/testcmp/cmd/testcmp
go tool testcmp ./...
```
