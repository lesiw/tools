# lesiw.io/tools/analyzers/typedefirst

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/typedefirst.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/typedefirst)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a type declared after the methods on it.

```go
package a

func (s *session) touch() {
    s.count++
}

type session struct { // type session should be declared before method touch
    count int
}
```

The compiler accepts either order, since a top-level type lives in the package
block. A reader opens the file to learn what the type holds, and meets a method
on it first.

Only a method creates the order. A method is declared on the type, so its
reader needs the declaration; a function that names the type, and a variable of
the type, only mention it. A method in another file gives no order to judge
against, so each file is read on its own.

The rule is adapted from go-critic's `typeDefFirst`. Upstream reports every
offending type in a declaration and offers no fix; this reports one type, lands
the diagnostic on it, and moves the declaration.

Fixable: the declaration, with its comment block, moves above the earliest
method on a type it declares, and the diagnostic names that method. The block
begins at the doc comment and every comment block one blank line above it, and
it ends with any comment that trails the declaration's last line. A comment
block one blank line above the method stays with the method, so the declaration
lands above it.

A `type (...)` declaration moves as a whole. The fix is withheld when the
declaration mentions a name declared between the method and itself, since a
declaration reads best after the names it mentions and the move would put it
above one; the diagnostic is reported without a fix. The same holds when the
file has changed since it was parsed.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/typedefirst/cmd/typedefirst
go tool typedefirst ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -typedefirst ./...
go tool lesiw.io/tools/cmd/fix -typedefirst ./...
```
