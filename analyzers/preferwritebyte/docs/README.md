# lesiw.io/tools/analyzers/preferwritebyte

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/preferwritebyte.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/preferwritebyte)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a one-element byte slice literal written to
a writer, and that literal converted to a string.

```go
func write(b *bytes.Buffer) {
    b.Write([]byte{'a'})               // one-byte slice written to a writer; use WriteByte
    b.WriteString(string([]byte{'a'})) // one-byte string written to a writer; use WriteByte
}
```

`io.Writer`'s `Write` writes "len(p) bytes from p" to the underlying data
stream, and `io.StringWriter`'s `WriteString` writes the bytes of the string.
The spec's conversions give a byte slice and a string the same bytes, so
writing one byte through either builds a one-element slice or a one-byte
string. `io.ByteWriter`'s `WriteByte` takes the byte.

The rule assumes `Write`, `WriteString` and `WriteByte` write the same bytes,
which `io.ByteWriter`'s doc does not require of an implementation.

The rule reports `Write` called with a byte slice literal of one element with
no key, whose element type is the predeclared `byte`, and `WriteString` called
with that literal converted to a string. A keyed element supplies its index,
so an element count is not a byte count.

The call is reported where the method it calls has `io.Writer`'s `Write` or
`io.StringWriter`'s `WriteString` signature, and the receiver also has a
`WriteByte` with `io.ByteWriter`'s signature, declared with the same receiver
form and reached through the same embedded fields. A value receiver does not
match a pointer receiver, since a write through a value receiver goes to a copy
of it.

When the receiver is an interface or a type parameter, the receiver form cannot
be compared: an interface flattens its embedded interfaces, and a type
parameter takes its methods from its constraint. The rule then assumes the
value's `Write` and `WriteByte` write to the same place, as it assumes they
write the same bytes.

A call is also skipped where:

- it does not take one argument, as with a method expression, which passes the
  receiver first, or `io.WriteString(w, s)`;
- the `WriteByte` it would call is declared by the caller's package, as a local
  type or interface does, where the rewrite could make `WriteByte` call
  itself;
- the file is a test file and the package it tests declares the method, since a
  test or an example may be exercising it.

Four shapes are left alone:

- `[]byte(v)` with a string `v`. `v` may hold more than one byte or none:
  `v[0]` panics on an empty string where `Write([]byte(v))` writes nothing. A
  one-byte string literal becomes a rune literal: `Write([]byte("'"))` becomes
  `WriteByte('\'')`. Where the receiver also has a `WriteString`,
  `stringxbytes` and `preferstringwriter` rewrite that call to `WriteString`.
- `string(v)` with an integer `v`. `string(v)` encodes `v` as UTF-8, which is
  one byte only below `utf8.RuneSelf`, so `WriteByte(byte(v))` would write one
  byte where the converted string has two or more.
- a one-byte string literal, as in `WriteString("a")`: no slice literal or
  conversion builds it, so there is nothing to drop.
- the conversion of a rune slice, as in `string([]rune{'a'})`, whose string is
  more than one byte as soon as an element is not ASCII.
The rule is like go-critic's `preferWriteByte`, which reports a `WriteRune`
call whose rune literal is one byte; this rule reports the `Write` and
`WriteString` forms, and leaves `WriteRune` alone.

Fixable: the slice literal or the conversion is dropped and the call becomes
`WriteByte`. `WriteByte` returns only an error, so the fix is withheld where
the call is not the whole statement, as in `return w.Write([]byte{c})` or
`n, err := w.Write([]byte{c})`, and where it would delete a comment.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/preferwritebyte/cmd/preferwritebyte
go tool preferwritebyte ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -preferwritebyte ./...
go tool lesiw.io/tools/cmd/fix -preferwritebyte ./...
```
