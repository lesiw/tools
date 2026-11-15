# lesiw.io/tools/analyzers/docstub

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/docstub.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/docstub)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a doc comment that only repeats the
declaration's name.

```go
package a

// Count returns the count.
func Count() int { return 0 } // doc comment only repeats the name Count

// An Item is an item.
type Item struct{} // doc comment only repeats the name Item
```

[Go's page on doc comments](https://go.dev/doc/comment) asks a function's doc
comment to explain what it returns, or what it does when it is called for its
side effects, and asks the same of a method. It asks a type's doc comment to
explain what each instance represents or provides. A comment built out of the
name answers none of those questions.

The comment has to begin with the whole name, or with one filler word and the
name. The words that carry nothing are dropped: the articles, `is` and `are`,
the verbs an author writes in place of a description (`does`, `returns`,
`represents`, `creates`, `constructs`), and the placeholders `whatever` and
`xxx`. What follows the name is then nothing, or the name again, or the name
with its last word inflected (`Fill fills`, `Copy copies`).

A character other than a letter, a digit, or a space character keeps a comment,
unless it is the last character, so a list item (`- Bullet`), a heading
(`# Unit`), a divider (`-- Value`), a doc link (`[Kind] is a kind.`), and a dot
or an arrow between the words (`Header.Type`, `Read -> Write`) all pass. A word
the list does not carry keeps the comment too, so `Close closes the connection`
and `ErrStop is the end` pass. A comment that names part of the name passes as
well (`A SplitKey is a key`), and so does one that repeats only the last word of
a multi-word name (`Reader returns the reader` above `OpenReader`). The words
are compared after splitting camel case, so `HTTPClient is an HTTP client` is
reported.

A package-level function, method, type, const, or var is checked, exported or
not, and so is a method declared in an interface. A struct field is left alone,
because a field's comment is often a one-word label. So are a local declaration,
an embedded interface member, and the package comment. A blank declaration
(`func _()`) has no name to repeat, and a value that shares a specification
with another (`const Min, Max = 1, 10`) has no single name. A comment above a
group of more than one declaration is left alone too: it documents the group.

A comment that shares its group with a directive, or that has no space after
its slashes, is left alone.

The rule is adapted from go-critic's docStub, which reports an exported type or
function documented as its name and a placeholder. This one checks every
package-level declaration and interface method, exported or not, and treats the
placeholders as one more word that carries nothing, so `A Widget xxx.` is
reported as well.

Fixable: the comment's lines are removed, leaving the declaration without a doc
comment. The report is suppressed, and the fix withheld, when deleting the line
would take something else with it: a directive or a no-space line in the same
group, or another comment that ends on the first line.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/docstub/cmd/docstub
go tool docstub ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -docstub ./...
go tool lesiw.io/tools/cmd/fix -docstub ./...
```
