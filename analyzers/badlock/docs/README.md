# lesiw.io/tools/analyzers/badlock

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/badlock.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/badlock)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a deferred call that does not release the
lock taken in the statement before it.

```go
func f(mu *sync.RWMutex) {
    mu.Lock()
    defer mu.RUnlock() // the deferred RUnlock does not release the lock taken by Lock
}
```

`Unlock`'s documentation says "It is a run-time error if m is not locked on
entry to Unlock", and `RWMutex`'s says "A RWMutex.RLock cannot be upgraded into
a RWMutex.Lock, nor can a RWMutex.Lock be downgraded into a RWMutex.RLock". A
deferred release that names the other method is a run-time error. A deferred
call that takes a lock waits for the lock the function already holds, and a
deferred `RLock` after an `RLock` waits only when a writer is blocked, and
otherwise leaves a read lock that nothing releases.

The rule reads a statement that calls `Lock` or `RLock` on a receiver, followed
directly by a `defer` of `Lock`, `RLock`, `Unlock`, or `RUnlock` on the same
receiver, in the same statement list. It reads the method names and not the
type, so a receiver of the author's own with a `Lock` and an `Unlock` is read
like a `sync.Mutex`. A receiver is compared by the object each name refers to:
`mu` and `(mu)` are one receiver, a second variable assigned the same pointer
is a second receiver, and two calls of `m.get()` are two receivers.

A method under another name is not read either: `mu.TryLock()` may return
without the lock, so `mu.TryLock()` followed by `defer mu.Unlock()` is left
alone. A call of one of those names that takes an argument is left alone in
either position, and so is a field of function type named `Lock`: the rule
reads method calls only.

A deferred function literal is left to `deferunlambda`, which rewrites `defer
func() { mu.RUnlock() }()` as `defer mu.RUnlock()`.

The rule is adapted from go-critic's `badLock`, which reads the same four
deferred pairs. It also reports a lock released in the following statement;
this rule reads deferred calls only, since one taken and released at once may
be measuring the lock or forcing a barrier. It adds `Lock` followed by a
deferred `RLock` and `RLock` followed by a deferred `Lock`; staticcheck's
SA2003 reports the same pairs that repeat the method, and only on `sync`'s own
mutexes.

Fixable: the deferred method becomes the release of the lock that was taken.
The fix rewrites the deferred call and never the lock statement: the rule
cannot see whether the function writes, so demoting a write lock to a read lock
could drop exclusion it relies on. It is withheld where the receiver's type has
no method with the release's name that takes no arguments, and where the
rewrite would take the line past the width `linelen` measures. A line already
past that width keeps the fix, since refusing it cannot shorten the line. With
no width set, nothing is withheld.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/badlock/cmd/badlock
go tool badlock ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -badlock ./...
go tool lesiw.io/tools/cmd/fix -badlock ./...
```
