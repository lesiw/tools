# lesiw.io/tools/analyzers/preferstringwriter

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/preferstringwriter.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/preferstringwriter)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a converted string written to a writer.

```go
type kind string

func write(b *bytes.Buffer, s string, k kind) {
    b.Write([]byte(s)) // converted string written to a writer; use WriteString
    b.Write([]byte(k)) // converted string written to a writer; use WriteString
}
```

Building the byte slice and then writing it copies the string in between.
`WriteString(s)` takes the string, so the byte slice is never built. Where the
operand is a defined string type, the fix writes `WriteString(string(k))`.

`io.WriteString`'s doc invokes a `StringWriter`'s `WriteString` directly and
calls `Write` "exactly once" otherwise. The rule assumes the two write the same
bytes, which `io.StringWriter`'s doc does not require of an implementation.

A call is reported where one type outside the caller's package declares both
methods with the `io.Writer.Write` and `io.StringWriter.WriteString`
signatures. A type that embeds one method and declares the other is skipped,
since the two can write different bytes, and so is a type the caller's package
declares, since the rewrite can land back inside its own `WriteString`. It is
also skipped where:

- neither method is declared by a named type, as with an inline interface,
  where the same re-entry cannot be ruled out;
- the receiver cannot take its address, where `WriteString` is a pointer
  method.

`preferfprint` reports the same call when the operand is a formatted string,
and the suite settles on its `fmt.Fprintf` replacement.

The rule is adapted from go-critic's preferStringWriter; this one leaves
`io.WriteString(w, s)` alone, since that call already dispatches to
`WriteString`.

Fixable. The fix is withheld where the edit would delete a comment.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/preferstringwriter/cmd/preferstringwriter
go tool preferstringwriter ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -preferstringwriter ./...
go tool lesiw.io/tools/cmd/fix -preferstringwriter ./...
```
