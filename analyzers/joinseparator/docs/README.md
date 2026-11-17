# lesiw.io/tools/analyzers/joinseparator

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/joinseparator.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/joinseparator)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a literal element ending with a redundant
separator in a `path.Join` or `filepath.Join` call.

```go
func config() string {
    return filepath.Join("dir", "alpha/") // filepath.Join element ends with a redundant separator
}
```

`filepath.Join("dir", "alpha/")` and `filepath.Join("dir", "alpha")` name one
path, so the fix removes the separator `alpha/` ends with.

## Checks

Join cleans its result, so a separator an element ends with leaves the string
the call returns unchanged. Only a whole literal element is read, because the
fix edits the literal in place. A separator inside a longer expression is not
one the fix can remove, and in `filepath.Join(root, "some-dir/"+kind)` it does
not end the value at run time either.

A separator is a byte that separates elements of the join: `/` on every host,
and for `filepath.Join` the host's separator too. The host is the machine
running the analyzer, so a tree built for another platform is read with this
one's separators. A backslash separates on Windows and is an ordinary
character elsewhere.

The last separator of an element is read. A first separator is not, because it
roots the value when every earlier argument is empty, and any earlier argument
may be empty at run time. `filepath.Join("dir", "/alpha")` reads the second
element as one name where `dir` is set, and `path.Join("", "/alpha")` reads it
as `/alpha`.

What is not reported:

- A value holding only separators, which may be the root. `filepath.Join("/",
  name)` is `/name` and `filepath.Join("", name)` is `name`, so the root has to
  stay.
- A volume. `filepath.Join("C:/", name)` is `C:\name`, and
  `filepath.Join("C:", name)` is `C:name`.
- A call that does not resolve to `path.Join` or `filepath.Join`, such as a
  field named `Join`, a method named `Join`, a variable holding
  `filepath.Join`, or `strings.Join`.

On Windows the separator of one element can decide the string an earlier one
produces: `filepath.Join("\\host", "share/")` is `\\host\share\`, because
`Clean` keeps the separator a share root was written with. An element is
therefore reported only when every argument is a constant and the host's `Join`
agrees, so `filepath.Join(dir, "alpha/")` is left alone there.

## Fixes

The suggested fix deletes the separators in place, so the rest of the literal's
spelling survives. When the cut lands inside an escape, the fix writes a fresh
quoted literal: `"alpha\u002f"` becomes `"alpha"`.

The rule is adapted from go-critic's `filepathJoin`, which reports any literal
argument of `filepath.Join` that holds a separator; this one reads a trailing
separator in `path.Join` or `filepath.Join` and carries a fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/joinseparator/cmd/joinseparator
go tool joinseparator ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -joinseparator ./...
go tool lesiw.io/tools/cmd/fix -joinseparator ./...
```

The rule follows the separator of the host it runs on, so a fixture that only
builds on Windows cannot pin it: the same file is analyzed on one platform and
ignored on another. The separator logic is covered by a unit test instead.
