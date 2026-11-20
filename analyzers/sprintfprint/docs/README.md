# lesiw.io/tools/analyzers/sprintfprint

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/sprintfprint.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/sprintfprint)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `fmt.Sprintf` call whose result a print,
log, or testing call formats again.

```go
fmt.Print(fmt.Sprintf("%d", n))   // Sprintf result passed to fmt.Print; use fmt.Printf
fmt.Println(fmt.Sprintf("%d", n)) // Sprintf result passed to fmt.Println; use fmt.Printf
t.Error(fmt.Sprintf("%d", n))     // Sprintf result passed to t.Error; use t.Errorf
```

The fmt package doc separates the calls by what they do with a format. `Print`
formats its operands with the default formats, and a string is its own default,
so `Sprintf`'s result reaches the output unchanged. `Printf` takes the format
and the operands, and writes that same string. For `Print`, these two write the
same bytes:

```go
fmt.Print(fmt.Sprintf(format, operands...))
fmt.Printf(format, operands...)
```

The calls checked are `Fprint`, `Fprintln`, `Print`, `Println`, and `Sprintln`
in `fmt`; `Print`, `Println`, `Fatal`, `Fatalln`, `Panic`, and `Panicln` in
`log`, package-level and on a `*log.Logger` or a type that embeds one; and
`Error`, `Fatal`, `Log`, and `Skip` in `testing`, on a `*testing.T`, a
`*testing.B`, a `*testing.F`, a `testing.TB`, or a type that embeds one. The
callee comes from the type checker, so an aliased import and a dot import are
both covered, and a method expression such as `(*testing.T).Error` is reported
with its receiver as the first argument. The rule reads the call, not the
format, so a format held in a variable is reported too. `fmt.Sprint` is reported
by `redundantsprint`, which drops the same wrapper.

The f form must resolve to the package that declares the call. A type that
embeds `*testing.T` and declares its own `Errorf` keeps that method, so the
rewrite would change which method runs and the call is left alone. A
dot-imported name is read at the call site, where a local declaration can
shadow it, and the call is left alone there too.

A call that passes a second message along with the `Sprintf` is left alone,
because the rewrite would have to move each operand into the format. A
`Sprintf` whose arguments come from one multi-value call is reported without a
fix where a writer comes first or a newline is appended, since that call fills
an argument list of its own.

`fmt`'s and `log`'s `ln` calls, and the four testing methods, always end the
message with a newline, so the fix writes one into the format. It writes `\n`
before the closing quote of an interpreted string literal, and concatenates
` + "\n"` to any other format:

```go
fmt.Println(fmt.Sprintf("%d", n))   // fmt.Printf("%d\n", n)
fmt.Println(fmt.Sprintf(format, n)) // fmt.Printf(format+"\n", n)
fmt.Println(fmt.Sprintf(`%d`, n))   // fmt.Printf(`%d`+"\n", n)
t.Log(fmt.Sprintf("%d\n", n))       // t.Logf("%d\n\n", n)
```

The fix adds one newline either way, even when the formatted message already
ends with one. testing's `Log` formats its arguments the way `Println` does,
and its `Logf` adds "a final newline ... if not provided", so the newline has
to be in the format for the two to record the same text.

The error markers that `printf` writes are the exception. Its `%!(EXTRA ...)`
marker follows the text the verbs produce, so a newline in the format comes
before the marker rather than after it, and a format that ends in `%` takes the
newline as its verb. `printf` reports those markers for a constant format only,
and this rule also reports a format held in a variable, so the marker that moves
can be one no tool has seen.

The rule is adapted from staticcheck's `S1038`, which reports these wrappers
without a fix, and leaves an `ln` call whose format is not a literal alone.

Fixable: the call is renamed to its f form, and the `fmt.Sprintf` around the
message goes. A comment inside the text the edit removes withholds the fix; the
call is still reported.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/sprintfprint/cmd/sprintfprint
go tool sprintfprint ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -sprintfprint ./...
go tool lesiw.io/tools/cmd/fix -sprintfprint ./...
```
