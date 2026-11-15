# lesiw.io/tools/analyzers/unnecessaryformat

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unnecessaryformat.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unnecessaryformat)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call whose format string has no verb.

```go
func f(w io.Writer) {
    fmt.Printf("done\n")        // format string has no verb; use Println
    fmt.Printf("done")          // format string has no verb; use Print
    fmt.Sprintf("some message") // format string has no verb; use Sprint
    fmt.Fprintf(w, "done\n")    // format string has no verb; use Fprintln
}

func g(t *testing.T) {
    t.Errorf("failed") // format string has no verb; use Error
}
```

The calls checked are `fmt.Printf`, `fmt.Sprintf`, `fmt.Fprintf`, and `Errorf`
on a type whose `Error` resolves to the method `testing` declares: `testing.T`,
`testing.B`, `testing.F`, `testing.TB`, or a type that embeds one. A type that
declares its own `Error` keeps the call, because the fix would call that
method.

`fmt.Errorf` is not among them. `errors.New` is the only rewrite, and the
suite's `errfmt` reports `errors.New` as replaceable by `fmt.Errorf`, so a fix
here would undo what `errfmt` asks for and leave a diagnostic no fix can clear.

The format must be a constant: for a value the analyzer cannot see, a
directive may be present, and `Print` copies what `Printf` interprets. The fix
rewrites a selector, so the callee must be written as one: a call through a
function value has no name to rewrite, and a dot-imported call has no selector.

A percent sign keeps the call, `%%` included, and no pass reports it:
`fmt.Printf("100%%")` prints one percent where `fmt.Print("100%%")` prints
two.

A call that passes arguments after the format string is left alone: fmt prints
them after the text as a `%!(EXTRA int=1)` marker, and the rename would print
them as ordinary operands. The `printf` pass reports the case where the format
is constant: a constant format with no verb and an argument. Neither pass
reports a format held in a variable with an argument after it.

The fix takes one trailing newline off the literal when the literal writes it
as `\n` and the text before it is not empty, and uses the `ln` name, which puts
it back: `fmt.Printf("done\n")` becomes `fmt.Println("done")`, and both write
the same bytes. The `ln` name is used only when the text before the newline
does not end in a space, a tab, a carriage return, or another newline; the
plain name keeps the text as written otherwise.

`Error` appends a newline of its own and has no `ln` form, so its text loses
the newline whenever the fix can take it off: `t.Errorf("x\n")` becomes
`t.Error("x")`, and `t.Errorf("x\n\n")` becomes `t.Error("x\n")`, which
records the same text. Where the fix cannot take the newline off -- the text is
a constant, the text is only a newline, or the literal spells the newline as
something other than `\n` -- the call is left whole, since `Error` would record
a newline that `Errorf` does not.

The rule is like revive's `unnecessary-format`, which reads a literal format
with no verb and leaves the edit to the reader. This analyzer makes the edit:
the rename is the one edit every call in the table can express.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unnecessaryformat/cmd/unnecessaryformat
go tool unnecessaryformat ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unnecessaryformat ./...
go tool lesiw.io/tools/cmd/fix -unnecessaryformat ./...
```
