# lesiw.io/tools/analyzers/octal

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/octal.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/octal)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an octal literal with a leading zero.

```go
const mode = 0755 // 0755 can be replaced by 0o755
```

Go 1.13 added the `0o` prefix, and both spellings compile to the same value.
The older spelling marks the base with a leading zero, which a reader expects
to be padding, so `0755` is easy to read as decimal.

The rule reads integer literals. A float literal stays decimal however many
zeros lead it: `0755.0` is 755.0. An imaginary literal is decimal too, because
the spec reads an integer part written in decimal digits as decimal even when
it starts with a zero: `0123i` is 123i. `gofmt` rewrites `0123i` to `123i`, so
a formatted file never holds one.

A literal is not reported when:

- It is an argument of a call that resolves to `time.Date`, whether the
  argument or the callee is parenthesized. The arguments are the fields of one
  date, and a padded field lines up with the ones beside it; the exemption
  covers every position, so it reaches the year and the nanosecond too. A
  literal inside an argument expression, as in `month(0644)`, is reported: only
  a literal written as the argument is exempt. A dot-imported `Date` is the
  same function, and a call through a variable is not.
- Its value is zero, since zero reads the same either way.
- Its separator is written straight after the zero, as in `0_755`: the
  separator sets the zero apart from the digits, so they do not read as a
  padded field.

A file is skipped when the language version the type checker records for it is
below Go 1.13, because that version cannot compile the prefix. The recorded
version is the module's, or the greater of the version the file's `//go:build`
line implies and Go 1.21, so only a file in a module below Go 1.13 is skipped.
A loader that supplies no version is treated as modern.

The fix inserts one byte and changes nothing else, so `0077` becomes `0o077`.
The zeros stay because padding can be deliberate: an octal header field is
padded to line up with the string beside it. A literal on a line already at the
width limit pushes the line one column over, so the author has to reflow it.

The rule is like go-critic's `octalLiteral`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/octal/cmd/octal
go tool octal ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -octal ./...
go tool lesiw.io/tools/cmd/fix -octal ./...
```
