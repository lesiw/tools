# lesiw.io/tools/analyzers/printbytes

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/printbytes.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/printbytes)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a byte slice printed by a call that formats
its operands the default way.

```go
fmt.Print(src)   // byte slice formats as a list of numbers; convert it to a string
log.Println(src) // byte slice formats as a list of numbers; convert it to a string
t.Log(src)       // byte slice formats as a list of numbers; convert it to a string
```

The fmt package doc says `Print` "formats using the default formats for its
operands". The default format of a slice prints its elements, so a byte slice
reaches the output as the decimal value of each byte: printing `[]byte("hi")`
writes `[104 105]`. Converting the operand to a string writes the text those
bytes encode.

The calls checked are `Fprint`, `Fprintln`, `Print`, `Println`, `Sprint`, and
`Sprintln` in `fmt`; `Fatal`, `Fatalln`, `Panic`, `Panicln`, `Print`, and
`Println` in `log`, package-level and on a `*log.Logger`; and `Error`, `Fatal`,
`Log`, and `Skip` in `testing`, on a `*testing.T`, a `*testing.B`, a
`*testing.F`, a `testing.TB`, or a type that embeds one. A type that embeds one
of those and declares the method itself keeps its own, so the callee is that
method and the call is left alone. The callee comes from the type checker, so
an aliased import, a dot import, and a method expression such as
`(*log.Logger).Print` are all covered.

The rule reads the operand's type, so a named byte slice, an alias of one,
and a slice whose element is a defined byte type are all reported. A type
parameter is not: its underlying type is its constraint rather than the slice
it stands for. A format-driven call is not either, because the verb chooses the
form: `%s` writes the bytes as text, and `%v` writes the numbers.

The rule leaves alone an operand whose type declares `String`, `Error`, or
`Format`: fmt calls one of those before it reaches the elements, so a type over
`[]byte` that declares one reaches the output as the method's text, and the
conversion would replace that text with the text the bytes encode. The method
must be in the value's method set, so a `String` method on the pointer does not
stop the report, and a pointer to the type is not a byte slice and is not
reported either.

The rule is adapted from staticcheck's `QF1010`, which reports a byte slice
printed by a call and wraps it in the same conversion.

Fixable: the operand is wrapped in a string conversion, which changes the
output from a list of numbers to the text the bytes encode. Nothing else in the
call changes. The fix is withheld where the name `string` reaches another
declaration; the operand is still reported.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/printbytes/cmd/printbytes
go tool printbytes ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -printbytes ./...
go tool lesiw.io/tools/cmd/fix -printbytes ./...
```
