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

Join cleans its result, so an element's trailing separator leaves the returned
string unchanged, unless it completes a root. Only a whole literal element is
read, because the fix edits the literal in place. A separator inside a longer
expression is not one the fix can remove, and in
`filepath.Join(root, "some-dir/"+kind)` it does not end the value at run time
either.

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

- A value holding only separators, which may be the root: `path.Join("/", name)`
  is `/name`; the edit would leave `path.Join("", name)`, which is `name`.
- A volume. On Windows, `filepath.Join("C:/", name)` is `C:\name` while
  `filepath.Join("C:", name)` is `C:name`, so `"C:/"` roots the path at the
  drive and `"C:"` does not.
- A call that does not resolve to `path.Join` or `filepath.Join`, such as a
  field named `Join`, a method named `Join`, a variable holding
  `filepath.Join`, or `strings.Join`.

On Windows a later element's trailing separator can change the result:
`filepath.Join("//host", "share/")` is `\\host\share\`, because `Clean` keeps
the separator written after a share root. A `filepath.Join` element is
therefore reported only when every argument is a constant and joining the
trimmed arguments produces the same path, so `filepath.Join(dir, "alpha/")` is
left alone there.

## Fixes

The suggested fix deletes the separators in place, so the literal keeps its
escapes and its quotes. When the cut lands inside an escape, the fix writes a
fresh quoted literal: `"alpha\u002f"` becomes `"alpha"`.

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
