# lesiw.io/tools/analyzers/bidicontrol

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/bidicontrol.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/bidicontrol)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a bidirectional control character in a
source file.

Go's [source code representation](https://go.dev/ref/spec#Source_code_representation)
says a source file is UTF-8, and the [unicode package](https://pkg.go.dev/unicode)
defines the characters the rule reports. These characters control the
direction of the text around them, so the displayed order can differ from the
order the compiler parses.

The rule is inspired by gosec's G116, which states the rendering hazard. The
Go documentation above establishes the encoding and the character property.

## Checks

### A bidirectional control character

The analyzer reports every character in Unicode's `Bidi_Control` property:
U+061C, U+200E, U+200F, U+202A through U+202E, and U+2066 through U+2069. It
reads every byte of the file, including comments and literals.

```go
func f() string {
    return "alpha‎ bravo" // a bidirectional control character U+200E
}
```

A character in a comment or in an interpreted string or rune literal is
replaced with an escape such as `\u202E`. In a literal the escape keeps the
value; in a comment the escape is text. A character in a raw string literal
has no fix, because an escape is literal text there. A character in a
directive comment has no fix, because the escape would change the directive.

An escape such as `\u202E` is the form that keeps a deliberate character in an
interpreted string or rune literal, and the analyzer does not report it. An
escape in a comment is also left alone.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/bidicontrol/cmd/bidicontrol
go tool bidicontrol ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -bidicontrol ./...
go tool lesiw.io/tools/cmd/fix -bidicontrol ./...
```
