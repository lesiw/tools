# lesiw.io/tools/analyzers/strreplaceall

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/strreplaceall.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/strreplaceall)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `strings.Replace` call with a count of
-1.

```go
func f(s string) string {
    return strings.Replace(s, "alpha", "beta", -1) // Replace with a count of -1; use ReplaceAll
}
```

The `strings.Replace` doc gives the count as the number of replacements and
states that a negative count removes the limit
([strings.Replace](https://pkg.go.dev/strings#Replace)).
[`strings.ReplaceAll`](https://pkg.go.dev/strings#ReplaceAll) replaces every
occurrence, so the count of -1 adds an argument and no information.

The rule is adapted from staticcheck's QF1004.

## Checks

The callee has to be `strings.Replace`, written with the package name. An
aliased import is covered. A call written without the package name is left
alone, as in a dot import, because the renamed call could resolve to a
different function. A method with the same name is left alone too.

A call through a function value is left alone because the value names
`Replace`, and `ReplaceAll` is a different function.

The count has to be the negative integer literal `-1`, with parentheses
allowed around it. A count in a constant or a variable is left alone, as in
`const all = -1` followed by `strings.Replace(s, old, new, all)`: the rule
reads the argument as written, and the fix would remove a name the reader may
want.

`bytes.Replace` is left alone. `strings.ReplaceAll` already means every
occurrence, so a call to it is not reported.

## Fixes

The fix renames `Replace` to `ReplaceAll` and deletes the comma and the count:
`strings.Replace(s, old, new, -1)` becomes
`strings.ReplaceAll(s, old, new)`. The rename changes only the function name,
so an aliased import keeps its alias. A comment between the last replacement
string and the count would be deleted with the count, so the fix is withheld
there and the diagnostic is still reported. A comment after the count stays.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/strreplaceall/cmd/strreplaceall
go tool strreplaceall ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -strreplaceall ./...
go tool lesiw.io/tools/cmd/fix -strreplaceall ./...
```
