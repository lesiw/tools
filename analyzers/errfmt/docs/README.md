# lesiw.io/tools/analyzers/errfmt

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errfmt.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errfmt)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` for error format strings: error
construction routes through `fmt.Errorf`, and error wrapping
routes through its `%w` verb.

`errors.New` can neither format nor wrap; `fmt.Errorf` does both,
is imported nearly everywhere already, and one error constructor
is simpler than two.

## Checks

### errors.New can be replaced by fmt.Errorf

```go
errors.New("connection lost") // errors.New can be replaced by fmt.Errorf
```

Reported only: a message could contain a percent sign `Errorf`
would interpret, so the rewrite is the author's work.

### Errors wrap with %w

An error value formatted into another error can be wrapped: `%v`
and `%s` verbs with error arguments become `%w`.

```go
fmt.Errorf("open failed: %v", err) // error can be wrapped with %w, not %v
```

The fix rewrites the verb in place, and is withheld when the
format's source spelling differs from its value, as with escape
sequences. Only arguments whose static type is `error` are
considered, and only literal format strings are inspected. An
indexed or star verb makes the argument mapping non-positional and
skips the whole call.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errfmt/cmd/errfmt
go tool errfmt ./...
```
