# lesiw.io/tools/analyzers/deferunlock

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deferunlock.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deferunlock)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a deferred Unlock without Lock, or a
deferred RUnlock without RLock, in the same function.

```go
func release(mu *sync.Mutex) {
    defer mu.Unlock() // deferred Unlock has no matching Lock in this function
}
```

sync.Mutex.Unlock's documentation says "It is a run-time error if m is not
locked on entry to Unlock", and sync.RWMutex.RUnlock's says the same for a read
lock (https://pkg.go.dev/sync#Mutex.Unlock,
https://pkg.go.dev/sync#RWMutex.RUnlock). The Defer statements section of the
spec says a deferred call runs when the surrounding function returns
(https://go.dev/ref/spec#Defer_statements). If the function has not taken the
lock, the deferred call runs with no lock held. A sync.Mutex or sync.RWMutex
release without a lock is a run-time error; another type with Unlock or RUnlock
may not fail.

The deferred call is checked when it selects a method named Unlock or RUnlock
and passes no arguments. The method may be on a mutex from sync or on another
type with those methods.

A matching lock is a call with no arguments to Lock on the receiver of an
Unlock, or to RLock on the receiver of an RUnlock. The other pair does not
match. A TryLock or TryRLock counts when its result is used, as in an if
condition or a use of the result. A TryLock or TryRLock counts unless the call
is a bare expression statement or its result is assigned to the blank
identifier, directly or inside another expression. A result passed to another
call counts as used. A TryLock that returns false still counts; the result is
not checked.

The call may appear before or after the defer, and it may be on a branch that
does not run. A deferred Lock counts only when it is declared after the
deferred release, so it runs first. A branch that may not run still counts, and
a deferred literal whose body is a single lock call counts at the defer
statement. A lock taken in a goroutine does not count.

Only calls in the function body count. A lock that the caller or a helper
takes on the receiver is not a matching lock, and the release is reported.

Locks and releases are not paired, so a lock that another release also unlocks
still matches.

Identifiers and fields are compared by object, with parentheses, address-of
expressions, and dereferences ignored. A pointer alias to a variable matches a
lock on the original. An address-of alias follows the aliased value. A value
copy does not. An alias to a field or an index, or from a type assertion, is
not followed. An assignment to a value base after the earlier of the lock and
the release stops a match. An assignment to a pointer base between the two
calls stops a match, because a deferred method value saves its receiver. Alias
assignments are resolved at the position of the lock and the release. For a
receiver behind an index or a type assertion, only an assignment between the
two calls stops a match. An index expression is compared structurally; a change
to the index variable between the calls is not tracked.

A receiver whose base value's last assignment before the defer came from a
call is left alone, because the call may return with the lock held. The base
value is followed through fields, indexes, dereferences, and type assertions.
A receiver that is itself a call, or a field, index, dereference, or type
assertion on a call, is left alone. A conversion and a builtin such as new or
make are not calls to a function that can return a locked value. Assignments
to the base value count. Assignments to a field or an index are left out.

A nested function literal is a separate function. A deferred call inside one is
checked against the locks in that literal, and a lock inside a literal does not
match a defer outside it. `deferunlambda` reports a deferred literal whose body
is a single call, which covers the Unlock and RUnlock case.

The diagnostic is placed on the method name.

The rule matches by method name, so a type whose release is valid without a
lock is reported.

No fix is offered: removing the defer and taking the lock earlier are both
repairs, and they leave the function in different states.

The rule is adapted from staticcheck's SA5001 family of checks for deferred
calls. SA5001 reports a deferred Close before the error from the open is
checked.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deferunlock/cmd/deferunlock
go tool deferunlock ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deferunlock ./...
```
