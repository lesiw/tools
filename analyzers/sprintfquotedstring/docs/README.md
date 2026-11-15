# lesiw.io/tools/analyzers/sprintfquotedstring

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/sprintfquotedstring.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/sprintfquotedstring)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a format string that quotes a `%s` by hand.

```go
fmt.Sprintf("\"%s\"", s) // format string quotes a %s; use %q
```

The fmt doc calls `%s` "the uninterpreted bytes of the string or slice", so a
quote in the value ends the quoted text early and a backslash in it starts an
escape. `%q` is "a double-quoted string safely escaped with Go syntax", quotes
included.

The calls checked are the printf-style functions that take a format string:
`Appendf`, `Errorf`, `Fprintf`, `Printf`, and `Sprintf` in `fmt`; `Fatalf`,
`Panicf`, and `Printf` in `log`; `Errorf`, `Fatalf`, `Logf`, and `Skipf` in
`testing`. The callee comes from the type checker, so an aliased or dot import
is covered, as is a printf method on a `*log.Logger` or a `testing.TB`. A call
through a function value is not examined.

A format is reported when all of these hold:

- The format string is a single literal written at the call. A format held in a
  constant or a variable is left alone: rewriting it would change every call
  that shares it.
- The format holds one verb, and that verb is spelled `%s` with nothing between
  the percent sign and the `s`. A width is left alone because it pads the value
  between the quotes and `%q` pads outside them. A precision or an argument
  index is left alone because the fix writes `%q` in place of the whole quoted
  verb. A second verb is left alone as well: the argument the rule reads is the
  one right after the format, which is the verb's argument only when the verb
  comes first.
- A double quote sits on each side of the verb, and no equals sign sits in
  front of the opening one, spaces or tabs aside.
- The argument the verb uses has a string, byte slice or byte array type,
  which `%q` writes as a quoted string. An integer keeps its `%s`, because the
  fmt doc says `%q` writes "a single-quoted rune literal" for one. An interface
  type is left alone because the dynamic type is unknown, and a type parameter
  is left alone whatever its constraint.
- The argument's method set has no `Format`: an `fmt.Formatter` receives the
  verb and decides for itself what it writes. A value whose pointer type has
  one is reported, since the value itself does not implement the interface.

A quote that follows an equals sign, spaces or tabs aside, is left alone, since
the field it closes may escape its own way, and `%q` escapes for Go.

The fix replaces the quotes and the verb and changes nothing else, so a raw
literal stays raw and a quote written as an escape is replaced too:
`"\x22%s\x22"` becomes `"%q"`.

The rule is adapted from go-critic's `sprintfQuotedString`, which matches the
quoted `%s` in the format's source text, so it also fires when the format holds
other verbs, and which reports without a fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/sprintfquotedstring/cmd/sprintfquotedstring
go tool sprintfquotedstring ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -sprintfquotedstring ./...
go tool lesiw.io/tools/cmd/fix -sprintfquotedstring ./...
```
