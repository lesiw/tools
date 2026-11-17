# lesiw.io/tools/analyzers/dynformat

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dynformat.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dynformat)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a printf-style call with a non-constant
format string and no argument after it.

```go
func f(s string) {
    fmt.Printf(s) // format string is not constant; use Print
}
```

The fmt doc describes Printf as formatting according to a format specifier and
Print as formatting its operands with their default formats. The suite's
printf pass reports a non-constant format string with no other arguments in a
call to a printf-style function in a file that declares Go 1.24 or later. The
pass doc says such a call may have been intended for the function's Print-like
counterpart; with no argument after it, the plain function or method leaves
the string unchanged. This analyzer has no language-version gate: the rule
applies to every file it checks.

The fix renames the function or method in the call to the plain one that takes
no format string:

- `fmt.Printf`, `fmt.Sprintf`, `fmt.Fprintf`, and `fmt.Appendf` become
  `Print`, `Sprint`, `Fprint`, and `Append`;
- `log.Printf`, `log.Fatalf`, and `log.Panicf` become `Print`, `Fatal`, and
  `Panic`, on the package and on a `*log.Logger`.

The callee is resolved by the type checker, so a call through an aliased
import, a method on an embedded `*log.Logger`, and a method expression are
covered. The call is
reported when all of these hold:

- The format string is not a constant expression. Any constant expression is
  left alone, including a literal, a named constant, or a concatenation.
- The call passes no argument after the format string. In a method expression
  the receiver is the first argument, so the call has one more argument ahead
  of the format.
- The callee is written as a selector, so the fix can rewrite its name. A
  dot-imported call is left alone: it has no selector, and a bare identifier
  could be shadowed by a local declaration.
- The plain name resolves to a function or method in the package that declares
  the formatted one. When a type declares a `Print` method, for example, the
  call is left alone, because the rename would call that method.
- A multi-value call used as the call's only argument is reported only when
  its values end at the format. A value after the format may be an argument,
  so such a call is left alone.

A call whose only argument is a multi-value call that ends at the format, as
in `fmt.Fprintf(pairWriter())` or `fmt.Appendf(pairBuffer())`, is reported and
fixed: the tuple supplies the writer, receiver, or buffer and the format, with
no argument after the format. A call through a function value is not examined.
A call through an interface method is not reported: the callee is an interface
method, so it is not one of the fmt or log functions in the table.

The fixed call writes the value as text. To format a value, pass it as an
argument: `fmt.Printf("%s", s)`.

Where the multi-value call is the format argument, as in `fmt.Sprintf(pair())`,
the printf pass has no multi-value guard. At Go 1.24 and later its fix inserts
`"%s", ` before that call, which does not compile; `cmd/fix` stops with a
package that does not load. This analyzer leaves the call alone.

For a call whose format is a single expression, the printf pass reports the
same call in a file that declares Go 1.24 or later and suggests inserting
`"%s", `. `cmd/vet` prints both diagnostics. The multi-value calls this
analyzer reports get no printf diagnostic: printf finds no format argument at
its format index. The suite sorts analyzers by name, so dynformat is analyzed
before printf. When no earlier fix covers part of the call, dynformat's
whole-call edit is applied and printf's insertion, which is inside that edit,
is skipped for the round.

The testing `Logf`, `Errorf`, `Fatalf`, and `Skipf` methods are not among the
calls. Their plain forms record through `Sprintln`, which adds a newline
before the final newline is trimmed, so a value that ends in a newline gains a
blank line.

`runtime/trace.Logf` is left out: `Log` reads the context before checking
whether tracing is enabled, while `Logf` returns first. The rename changes
behavior with tracing off.

`fmt.Errorf` is left alone. Its rewrite is `errors.New`, in another package, so
a rename cannot write it.

In a file that declares Go 1.24 or later, the printf pass reports the testing
and trace calls and `fmt.Errorf`; its `"%s"` fix writes the value as text.

The rule is adapted from staticcheck's `SA1006`, which reports a dynamic
format string with no further arguments and rewrites the call. This analyzer
covers the fmt and log calls above.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dynformat/cmd/dynformat
go tool dynformat ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dynformat ./...
go tool lesiw.io/tools/cmd/fix -dynformat ./...
```
