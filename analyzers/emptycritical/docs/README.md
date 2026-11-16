# lesiw.io/tools/analyzers/emptycritical

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/emptycritical.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/emptycritical)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a lock taken and released with no
statement between the two calls.

```go
func f(mu *sync.Mutex) {
    mu.Lock()
    mu.Unlock() // empty critical section
}
```

`sync.Mutex`'s documentation says "The zero value for a Mutex is an unlocked
mutex", and `Unlock`'s says "A locked Mutex is not associated with a particular
goroutine". A section that takes the lock and gives it back does no work
between the calls. The author often meant the `Lock` to cover the work below
it, or wrote an `Unlock` without the `defer` they meant.

The rule applies to two statements in the same block, case body, or comm
clause body. Both must be calls of a method that takes no arguments and
returns no results. The method names must be `Lock` followed by `Unlock`, or
`RLock` followed by `RUnlock`. The receivers must read the same value: `mu`
and `(mu)` are one receiver, a second variable assigned the same pointer is a
second receiver, and two calls of `get()` are two receivers.

A method with those names on another type is read like a `sync.Mutex`. A field
of function type named `Lock` is not a method, and neither is a package-level
function with that name, so neither is reported. The rule does not report
`TryLock` or `TryRLock`: either may return without the lock. It does not
report a `defer` in the second statement, since its release comes at the end
of the function and work may follow the pair.

Not reported:

- a statement between the two calls;
- two calls on receivers that do not read the same value;
- `Lock` followed by `RUnlock`, or `RLock` followed by `Unlock`;
- a call that takes an argument or returns a result;
- a receiver written as a call, since the two calls can return different
  values;
- a pair in a test file.

Report only: `Unlock` and `RUnlock` "synchronize before" the next `Lock`, so
either pair can be a barrier another goroutine waits on, and removing it would
change the program. The repair is either to move the work inside the pair or
to defer the release, and the statements do not show which repair the author
intended. An `//ignore:emptycritical` directive suppresses the report for a
deliberate barrier.

The rule is adapted from staticcheck's SA2001
(https://staticcheck.dev/docs/checks/#SA2001), which reports the same pair and
offers no fix. SA2001 reports a receiver written as a call, and this rule does
not report it; SA2001 checks block statements only, and this rule checks case
and comm clause bodies too. SA2001 skips the `sync` package's tests; this rule
skips every test file.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/emptycritical/cmd/emptycritical
go tool emptycritical ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -emptycritical ./...
go tool lesiw.io/tools/cmd/fix -emptycritical ./...
```
