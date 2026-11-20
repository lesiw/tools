# lesiw.io/tools/analyzers/yield

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/yield.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/yield)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to a yield function that may be
reached after an earlier call returned false.

```go
func sequence(yield func(int) bool) {
    yield(0) // yield may be called again (on L3) after returning false
    yield(1)
}
```

A range statement's own yield panics with "range function continued iteration
after function for loop body returned false" when the iterator calls it again,
which is why the spec says it must not be.

## Checks

The check is a "may" analysis over the function's control flow. It carries,
along each edge, the yield calls that may have returned false on the way there,
each with the conditions the path took on local booleans: a constant, a yield
result, a negation, or the merge of two paths. A "must" analysis would miss the
usual fault, which is a yield call with no check at all. The analysis cannot
see a condition held in a field or handed to another function, and treats it as
possibly false, so more calls are reported. A comparison is one of those: a
call after `if yield(0) == stop` is reported, because the analysis follows the
call's own result and not the boolean that a comparison makes of it.

Only a call written as a bare identifier named yield counts, with at most two
parameters and one `bool` result. The package must also import `iter`, which is
the signal that it defines iterators.

A path is not pruned on a constant condition, so a yield call inside `if false`
is still reported. A condition that is constant here is usually a build-time
choice such as a build tag or `GOOS`, and pruning on it would specialize the
result to one configuration.

## Exemptions

Not reported:

- a call that a check leaves unreachable, as when `if !yield(0) { return }`
  ends the iterator before the next call;
- a call in a `&&` chain, where each call runs only when the one before it
  returned true;
- a call from a `go` statement or a `defer` statement, since it does not run
  where it is written;
- every call in a package that does not import `iter`, including a package
  that declares its own iterator type and imports nothing, as gopls skips it
  too.

## Fixes

The repair is an `if` on the earlier call:

```go
func guarded(yield func(int) bool) {
    if yield(0) {
        yield(1)
    }
}
```

When each call is the whole of its own statement and the next statement in the
same block or clause is the next call, the run of calls is nested one inside
the next, so that each call runs only when the one before it returned true. A
run of any length that a loop body does not replay takes one edit, on the
diagnostic at the run's first call, and the diagnostics after it are reported
without one. A run a loop body replays is reported and left alone: the fault
there is the iteration, and the diagnostic names the call against itself.

Reported without a fix:

- a call that shares its line with other code, that has a comment or a blank
  line between it and the next call, or that is written across lines, since the
  edit would move text;
- a call in code indented with spaces, since the edit writes tabs;
- a call through a name other than the enclosing function's own parameter: a
  package-level function, a local, or a parameter a nested function captured,
  which reaches the call as a load. The rule cannot tell any of them from an
  iterator's own yield, and the guard would land in code that may not follow
  the protocol.

Vendored from gopls' `yield` (`gopls/internal/analysis/yield`,
`golang.org/x/tools/gopls` v0.24.0), which is internal and cannot be imported.
The rule, the analysis, the transfer functions, and the message are upstream's.
The fix is written here, because gopls reports only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/yield/cmd/yield
go tool yield ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -yield ./...
go tool lesiw.io/tools/cmd/fix -yield ./...
```
