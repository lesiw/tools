# lesiw.io/tools/analyzers/uncatchablesignal

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/uncatchablesignal.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/uncatchablesignal)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `signal.Notify` or
`signal.NotifyContext` call that names `SIGKILL` or `SIGSTOP`.

```go
func f(ch chan os.Signal) {
    signal.Notify(ch, syscall.SIGKILL) // syscall.SIGKILL cannot be caught
}
```

The `os/signal` package documentation says the two signals "may not be caught
by a program, and therefore cannot be affected by this package"
(https://pkg.go.dev/os/signal). Neither signal can be caught, so registering a
handler for one changes nothing.

The rule covers `syscall.SIGKILL`, `syscall.SIGSTOP`, and `os.Kill`. It follows
a signal through parentheses and a conversion to `os.Signal` or
`syscall.Signal`; a conversion to any other type is left alone. A signal is
recognized by the object it names, so one copied into a local variable, as in
`sig := os.Kill`, returned by a function call, or held in a slice, is left
alone. The rule covers `signal.Notify` and `signal.NotifyContext`, the calls
that relay a signal to a channel; `signal.Ignore` and `signal.Reset` are left
alone.

The fix removes the uncatchable signals and the comma that joins them to a
neighbor. A call that names several uncatchable signals is reported once. A
call whose signal arguments are all uncatchable is reported without a fix,
because removing every signal would register for every signal. Deleting the
call is not offered because the channel or context expression may have effects
and `signal.NotifyContext` returns values the caller uses.

Fixable when a catchable signal remains and no comment sits in or next to the
removed text; report only otherwise, because the edit would delete the comment
or leave it describing another signal.

On Plan 9, `os.Kill` and `syscall.SIGKILL` are the "kill" note, which the
runtime handles without notifying `os/signal`, so the rule applies there too.

The rule is adapted from staticcheck's SA1016, which reports `signal.Ignore`,
`signal.Notify`, and `signal.Reset`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/uncatchablesignal/cmd/uncatchablesignal
go tool uncatchablesignal ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -uncatchablesignal ./...
go tool lesiw.io/tools/cmd/fix -uncatchablesignal ./...
```
