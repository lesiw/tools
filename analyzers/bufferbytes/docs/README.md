# lesiw.io/tools/analyzers/bufferbytes

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/bufferbytes.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/bufferbytes)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `bytes.Buffer` converted to a string or
a byte slice.

```go
func text(buf *bytes.Buffer) string {
    return string(buf.Bytes()) // conversion of Bytes to a string; String returns it for a non-nil buffer
}
```

`bytes.Buffer`'s `String` returns the contents of the unread portion of the
buffer as a string (https://pkg.go.dev/bytes#Buffer.String), and `Bytes`
returns the same contents as a slice that aliases the buffer
(https://pkg.go.dev/bytes#Buffer.Bytes). For a non-nil buffer,
`string(buf.Bytes())` and `buf.String()` return the same string.

The rule is inspired by staticcheck's `S1030`, which reports both forms. The
byte-slice form is reported without a fix, and the map exemption is wider than
the one in `S1030`.

## Checks

`string(buf.Bytes())` is reported when the conversion target is the predeclared
`string` type or an alias of it, and `String` resolves to `bytes.Buffer.String`
on the receiver. `[]byte(buf.String())` is reported when the conversion target
is `[]byte` or an alias of it, and `Bytes` resolves to `bytes.Buffer.Bytes` on
the receiver.

Defined string and slice types are left alone. The rule matches conversions
whose target is `string` or `[]byte` (or an alias), where the method call can
replace the whole conversion; the conversion remains for a defined target. A
slice of a defined byte type, as in `[]octet`, is also left alone: `Bytes` returns
`[]byte`.

The called method and the alternative method both have to be declared by
`bytes.Buffer`. A wrapper that declares `String` or `Bytes` hides one of them.
A call through an interface is left alone too. The rule matches direct method
calls only. A parenthesized method value, as in `(buf.Bytes)()`, and a method
expression such as `(*bytes.Buffer).Bytes(buf)`, are left alone.

The receiver can be a `bytes.Buffer` or `*bytes.Buffer`, directly, through an
embedded field, or through a type alias.

## Exemptions

A conversion used as an index or map key is exempt. Map lookups and index
assignments are the exemptions in `S1030`; `delete`, composite-literal keys, and
conversions nested in a conversion or literal used as an index or key are also
exempt. The exemption is lexical: it does not check the key type, element size,
or compiler fast path.

```go
m[string(buf.Bytes())]
delete(m, string(buf.Bytes()))
m[string(buf.Bytes())] = 1
```

A conversion passed to the predeclared `len` is left to `stringxbytes`. So is
a conversion on the left of `==` or `!=` with the empty string literal on the
right, when `len` is predeclared.

```go
len(string(buf.Bytes()))
string(buf.Bytes()) == ""
```

`stringxbytes` also reports the byte-slice form in `copy`, an `append` with
the spread, and a `Write` whose receiver has `WriteString`, where the fix drops
or renames the conversion. Where both report the call, `stringxbytes` offers
the fix.

The `bytes` package is left alone, where a rewrite could call the method it is
in. Its external tests are left alone, where a rewrite could make a test
tautological.

## Fix

The string form is fixed when the receiver, or the embedded `bytes.Buffer`
field, is a value, as in a `bytes.Buffer` value or a wrapper that embeds one,
and the file is not a test file. A test may compare the conversion with the
method, and the rewrite would make that comparison tautological.

```go
func text(buf bytes.Buffer) string {
    return string(buf.Bytes()) // buf.String()
}
```

A pointer to `bytes.Buffer` is fixed by dereferencing it, as `(*buf).String()`,
or directly when the expression cannot be nil, as with `&v`,
`new(bytes.Buffer)`, `bytes.NewBuffer`, or `bytes.NewBufferString`. An embedded
`*bytes.Buffer` is fixed when each field on the path can be named, as
`(*w.Buffer).String()`. Both fixes are withheld in a test file. An unexported
field from another package is reported without a fix.

The byte-slice form is reported without a fix. `Buffer.Bytes` returns a slice
that aliases the buffer content, valid only until the next modification, while
`[]byte(buf.String())` returns a copy. Replacing one with the other changes
what a caller sees if it modifies or keeps the result. For a non-nil,
non-empty buffer, `bytes.Clone(buf.Bytes())` makes the same copy. A nil buffer
makes `Bytes` panic where the conversion copies `"<nil>"`, and when `Bytes`
returns nil, `Clone(nil)` is nil where the conversion gives an empty slice.

The fix is withheld when an edit would delete a comment.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/bufferbytes/cmd/bufferbytes
go tool bufferbytes ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -bufferbytes ./...
go tool lesiw.io/tools/cmd/fix -bufferbytes ./...
```
