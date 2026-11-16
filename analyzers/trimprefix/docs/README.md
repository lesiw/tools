# lesiw.io/tools/analyzers/trimprefix

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/trimprefix.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/trimprefix)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `HasPrefix` guard around a `TrimPrefix`
call of the same prefix assigned back to the same operand in a file before Go
1.20.

```go
func f(s, prefix string) string {
    if strings.HasPrefix(s, prefix) { // redundant HasPrefix guard around TrimPrefix
        s = strings.TrimPrefix(s, prefix)
    }
    return s
}
```

The [`strings.TrimPrefix`](https://pkg.go.dev/strings#TrimPrefix) doc says it
returns `s` unchanged when `s` does not start with `prefix`, and the
[`bytes.TrimPrefix`](https://pkg.go.dev/bytes#TrimPrefix) doc says the same. The
guard adds nothing to the trimmed value.

```go
s = strings.TrimPrefix(s, prefix)
```

The rule matches only the `HasPrefix` and `TrimPrefix` pair. A manual slice
after `HasPrefix` is left alone.

The rule is adapted from staticcheck's S1017, which reports a `HasPrefix`
followed by a manual slice and rewrites it to `TrimPrefix`. This rule reports a
`HasPrefix` guard whose test the `TrimPrefix` call already covers.

This rule applies to files before Go 1.20. At Go 1.20 and later, `modernize`'s
`stringscutprefix` reports the pair and rewrites it to `CutPrefix` in the
matching package, so a suite run does not report the same `if` twice. For a map
write, that rewrite keeps the guard, so it does not create an absent key. A
file with no version information is left alone. The `-trimprefix.modern=true`
flag reports the pair anyway.

Not reported:

- a file whose language version is Go 1.20 or later, which `stringscutprefix`
  reports, unless `-trimprefix.modern=true` is set;
- a file with no language version information, because there is no version to
  compare;
- an if with an init statement: removing the condition would also remove the
  init statement;
- an if that has its own else, or a body with more than one statement:
  removing the condition changes whether the other statements run;
- an assignment whose left side is not the operand the condition tests;
- an assignment whose write target is an index into a value that is not known
  to be a writable index target, because writing a map index creates an absent
  key, while skipping the write leaves the map alone;
- a trim whose operand or prefix the comparison cannot match: it covers
  identifiers, selectors, indexes, dereferences, and basic literals, not calls,
  composite literals, or larger expressions;
- a call that is not the `strings` or `bytes` package function, such as a
  method with the same name.

## Fixes

The fix removes the if header and closing brace and leaves the trim
assignment. An else-if keeps its braces: the fix removes only the if header,
leaving an else block.

No fix is offered when a comment lies between `if` and `{`, since the header
edit would delete it. A comment after `{` or in the body stays. If the source
cannot be read, the report stands without a fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/trimprefix/cmd/trimprefix
go tool trimprefix -trimprefix.modern=true ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -trimprefix -trimprefix.modern=true ./...
go tool lesiw.io/tools/cmd/fix -trimprefix -trimprefix.modern=true ./...
```
