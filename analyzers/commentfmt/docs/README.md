# lesiw.io/tools/analyzers/commentfmt

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/commentfmt.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/commentfmt)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comment with no space after its slashes.

```go
package a

//good is not how a comment starts. // comment has no space after //

// Fine is a normal comment.
func Fine() {}

//go:generate echo hi

//nolint:foo

//lint:ignore

// ---------- separator ----------

//x // comment has no space after //

func bad2() {} //trailing // comment has no space after //

//line other.go:10
func directive() {}

//export Exported
func exported() {}

//sysnb getgroups(ngid int, gid *_Gid_t) (n int, err error)

func sysnb() {}

//doc word
func docWord() {}

//.....a separator drawn with dots is not prose
//---- neither is one drawn with dashes
//.data is a section name
```

A comment with no space after its slashes looks like code that was commented
out, and editors highlight it that way. A space makes it prose.

A directive such as //go:generate, //line or //nolint has no space by design.
gofmt writes the space in a doc comment itself.

The idea comes from go-critic's commentFormatting.

Fixable, for a comment that is not a doc comment.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/commentfmt/cmd/commentfmt
go tool commentfmt ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -commentfmt ./...
go tool lesiw.io/tools/cmd/fix -commentfmt ./...
```
