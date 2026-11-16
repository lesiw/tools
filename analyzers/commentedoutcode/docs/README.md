# lesiw.io/tools/analyzers/commentedoutcode

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/commentedoutcode.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/commentedoutcode)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a block of commented-out code.

```go
package a

func load(names []string) {
	// println(names) // commented-out code
	// loadAll(names)

	// if len(names) == 0 { // commented-out code
	// 	return
	// }
}
```

[The spec](https://go.dev/ref/spec#Comments) says a general comment "acts like
a space", that any other "acts like a newline", and that comments "serve as
program documentation". A statement left in a comment is not type-checked, not
formatted, and not run by a test, so it can disagree with the code beside it
and nothing reports the disagreement. Version control keeps the statement.

A comment on one line is treated as a note. A longer comment inside a function
body is read as statements, and outside one as declarations.

A commented-out call or declaration is reported, and so is any statement with a
body of its own, so a commented-out `if` with a commented-out body is one
report. A `go` or `defer` statement counts, since it holds a call and nothing
else. A lone assignment or return is left alone: a comment states a formula or
a step that way.

A group that begins with a label is a heading over the lines below it, so it is
left alone; a colon makes the heading. So is a group holding a directive such
as `//go:generate`, since deleting it would remove a line a tool reads. A type
declaration is left alone at any size, since a comment shows a type's shape
that way, as `type count int` does. A function declaration counts only with a
body; without one it is a signature.

A comment that documents a top-level declaration is left alone, and so is the
comment on a field or a package. It describes the declaration below it, and the
examples in it belong to a reader. A comment that follows code on its line is
left alone as well, since it annotates the line it is on.

What the rule cannot see: a fragment of a composite literal (`// name: "x",`)
and a `case` clause, since neither parses on its own; a star-form block
comment, whose leading `*` makes each line a dereference; and a block of code
with a note on a line of its own inside it, since the note keeps the text from
parsing. A note that trails a code line on the same line does not keep the text
from parsing, so the group is reported. A comment that illustrates a transform
by writing the transform's statements is reported, since that text is code.

Report only: deleting the comment can take a note the author wrote on one of
its lines (`// item.save()` and `// item.reset() // clear the cache`), and the
analyzer cannot tell that note from the code. A note on a line of its own stops
the report; a trailing note does not.

The rule is inspired by go-critic's `commentedOutCode`, which reports a
comment inside a function body.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/commentedoutcode/cmd/commentedoutcode
go tool commentedoutcode ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -commentedoutcode ./...
```
