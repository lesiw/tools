# lesiw.io/tools/analyzers/filepathjoin

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/filepathjoin.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/filepathjoin)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a path concatenated with a literal
separator.

```go
func config(dir, name string) string {
	return dir + "/" + name // path concatenated with a literal separator can be replaced by filepath.Join
}
```

Package `filepath` documents the two facts the rule leans on: its routines
follow the operating system, and a path that always uses a forward slash, such
as a URL, belongs to the `path` package. `filepath.Join` separates its
elements with the platform's own separator. A concatenation writes one by hand:
it hardcodes the choice, and it leaves a second separator in place when the
element before it already ends with one.

A slash in an expression is more often part of a URL, a test name, or an
archive member than a filesystem path, so the rule reads the file's import of
`path/filepath` before it reports anything.

## Checks

A string literal holding one byte that `os.IsPathSeparator` accepts is a
separator, so a backslash separates on Windows and is an ordinary character on
Unix. The reported expression alternates elements with separators, begins with
an element, and ends the chain.

`dir + "/" + name` and `dir + "/" + sub + "/" + name` are reported.
`dir + "/"` and `"/" + name` are not, because one holds an element with no
separator after it and the other a separator with no element before it.
`dir + "/" + name + ".log"` is not either: the join drops a separator `name`
ends with, and the `.log` would then attach to the shortened element.

What is not reported:

- A file that does not import `path/filepath`, imports it under a name the fix
  cannot write, or has something else in scope with that name.
- A chain holding a literal that says the chain is not a plain path: a URL, a
  format string, a glob, or an empty string. The chain is the unit, so a
  literal anywhere in it keeps the whole expression quiet, even where the
  reported part alone would be a path. A `://`, a `%`, or one of `*?[` in an
  operand marks a form the join does not write, and an empty element is how a
  concatenation keeps a separator the join drops. An element that is a
  `filepath.Join` of no arguments reads the same way. A variable holding one of
  these cannot be told apart from one holding a directory name, and is
  reported.
- A chain of string literals, which is one literal written in pieces and is
  `uselessconcat`'s business.
- An element whose type is a named string type, or a type parameter.
  `filepath.Join` takes `string`, so the rewrite would need a conversion.
- A path between brackets that is one element of a longer expression, where
  the brackets hold a path of their own: `dir + (kind + "/" + name)`,
  `(dir + "/") + name`, and `dir + (kind + "/" + name) + "/" + tail` all
  report nothing. The rewrite would have to reach across brackets the path was
  written with. Brackets holding text that is not a path, as in
  `(dir + prefix) + "/" + name`, are one element and are reported.

## Fixes

The elements become the arguments of `filepath.Join`, named the way the file's
import of `path/filepath` does. An element that is already a `filepath.Join`
call contributes its own arguments, so `filepath.Join(dir, sub) + "/" + name`
becomes `filepath.Join(dir, sub, name)`.

`filepath.Join` Cleans its result and a concatenation does not: the rewrite
drops a separator an element ends with, and resolves away an element that is
`..`. The rewrite is the cleaned path, so a value that is empty or ends with a
separator at run time produces a different string afterwards, and the rule
treats the cleaned path as the one the code meant.

The report carries no fix for a path written across lines, for an expression
with a comment inside it that the rewrite would drop, and for a rewrite that
would push its line past the width `linelen` measures.

The rule is adapted from go-critic's `preferFilepathJoin`, which reports a
concatenation with `string(os.PathSeparator)`; this one reads a string literal
separator.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/filepathjoin/cmd/filepathjoin
go tool filepathjoin ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -filepathjoin ./...
go tool lesiw.io/tools/cmd/fix -filepathjoin ./...
```
