# lesiw.io/tools/analyzers/writestring

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/writestring.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/writestring)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to `WriteString` whose argument
concatenates strings.

```go
func write(first, second string) {
    var out strings.Builder
    out.WriteString(first + second)
}
```

The spec says string addition "creates a new string by concatenating the
operands" (https://go.dev/ref/spec#String_concatenation), and `WriteString` then
copies that string into the writer, so the bytes are copied twice. A call per
operand copies each part once.

The call is reported when the writer is a `bufio.Writer`, a `bytes.Buffer`, a
`hash/maphash.Hash`, or a `strings.Builder`. A call on an `*os.File` is left
alone: its `WriteString` writes the string in one system call, and on a socket
that call is one datagram, so splitting it could send two messages where the
author sent one. A call to `io.WriteString` is left alone as well, since the
method that runs may be an `*os.File`'s.

The call must stand alone in a statement list, labeled or not, so nothing reads
its two results and the split has somewhere to go. An `if`, `for`, or `switch`
header holds one statement where the fix writes two.

The receiver must be stable: a call in it must give the same value twice, and a
receive or the address of a literal such as `&T{}` makes it unstable. An operand
after the first must not name anything the receiver names either, since the
statements run in order and a later read would see what the earlier call wrote.

A subexpression the compiler folds is one operand, and the fix writes it whole.

The fix is withheld where a comment sits inside the call. The diagnostic is
still reported.

Fixable: the call becomes one call per operand.

The rule is vendored from gopls' `writestring`
(`gopls/internal/analysis/writestring`, `golang.org/x/tools/gopls` v0.24.0),
which is internal and cannot be imported. It is upstream's, except that this
reports a parenthesized argument and leaves alone a call in a header, a receiver
the split would evaluate to a new value, and an operand after the first that
names what the receiver names.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/writestring/cmd/writestring
go tool writestring ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -writestring ./...
go tool lesiw.io/tools/cmd/fix -writestring ./...
```
