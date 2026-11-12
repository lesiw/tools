# lesiw.io/tools/analyzers/unusedparams

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unusedparams.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unusedparams)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a parameter a function never reads.

```go
func unread(a int, b string) string { // parameter a is never read
	return b
}
```

A parameter that is never read is either a leftover or a signature the author
meant to keep. Renaming it to the blank identifier says which, and costs
nothing:

```go
func unread(_ int, b string) string {
	return b
}
```

The rule stands down where the name may be read or copied from elsewhere:

- an exported function or method, which is documented by its parameter names;
- an unexported method whose name matches an interface method in the package,
  which may be written to match it;
- any function used as a value rather than called, which may have to match a
  func type;
- a function with no body, or one that only panics, which is a placeholder.

A func literal's parameters are left alone: the signature it is assigned to is
written somewhere else, and the literal cannot show whether it was chosen.

This mirrors gopls' `unusedparams`
(`gopls/internal/analysis/unusedparams`, `golang.org/x/tools/gopls`), which is
internal and cannot be imported. The rule for a declared function is the same;
gopls also reports a func literal, which this does not.

## Usage

    go tool lesiw.io/tools/cmd/vet -unusedparams ./...
    go tool lesiw.io/tools/cmd/fix -unusedparams ./...
