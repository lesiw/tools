# lesiw.io/tools/analyzers/defermisuse

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/defermisuse.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/defermisuse)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a recover call that does not run directly
in a deferred function, and a return with results in a deferred literal.

```go
func f() {
	recover() // recover outside a deferred function has no effect
}

func g() {
	f()
}
```

The `recover` documentation says recover stops a panic when "executing a call
to recover inside a deferred function (but not any function called by it)"
(https://pkg.go.dev/builtin#recover). A recover call in the body of a function
that is not the deferred function does nothing.

The Defer statements section of the specification says "if the deferred
function has any return values, they are discarded when the function
completes" (https://go.dev/ref/spec#Defer_statements). A return inside a
deferred literal ends the literal; it does not return from the function that
deferred it.

## Checks

### Recover

A recover call is reported when it does not run directly in the body of the
deferred function.

At the defer statement:

```go
defer take(recover()) // recover runs when the defer is registered
```

A recover written as the deferred call itself is reported:

```go
defer recover() // recover is the deferred call
```

A defer inside a deferred function evaluates its arguments while that function
runs, so a recover in an argument works. A recover written as the deferred call
is still reported: it recovers only a panic that was already active when the
deferred function started, and calling recover directly is clearer.

Outside a defer:

```go
func f() {
	recover() // recover outside a deferred function has no effect
}

func g() {
	f()
}
```

In a function literal that the deferred function calls:

```go
defer func() {
	func() {
		recover() // recover outside a deferred function has no effect
	}()
}()
```

Silent, deliberately:

```go
defer func() {
	recover()
}()
```

A recover call directly in the body of a named function that a defer in the
same package calls is silent. So is a method whose interface method a defer
calls. A function literal stored as a value, a named function used as a value,
or a named function not used in the package is not checked, because a caller
can defer it. An exported function is not checked for the same reason. A main
or init function is checked, because the runtime calls it and a recover in that
invocation does nothing. A direct call through an interface with more than one
implementation is not followed, because the analyzer cannot tell which method
runs.

In a new goroutine, recover cannot stop a panic in the goroutine that started
it. A `go recover()` call is reported:

```go
go recover() // recover in a new goroutine has no effect
```

The function and arguments of a go statement are evaluated in the current
goroutine. In an ordinary function, `go take(recover())` is reported because
recover runs outside a deferred function. Inside a deferred function, that
function calls recover directly, so the call is silent:

```go
defer func() {
	go take(recover())
}()
```

### Return

A return with results directly in the body of a deferred literal is reported.

```go
defer func() error {
	return err // the return value is discarded
}()
```

Silent, deliberately:

- A bare return in a deferred literal. It ends the literal early, which is a
  useful way to skip the rest of the cleanup.
- A return in a function literal that the deferred literal calls, which
  returns from that inner function.
- A return in a named function, even when the defer calls that function.

A literal stored once in a variable and then deferred is checked:

```go
f := func() error {
	return err // the return value is discarded
}
defer f()
```

The assignment is found by position, so a package-level variable assigned in
another function is approximate.

## Report only

Each check reports without a fix. Moving recover into the deferred function
needs that function, and removing a return changes what the literal does after
it. Only the author knows which repair is meant.

Test files are not analyzed. A function used only from a test has no use
outside the test files, so it is not checked. A function the code calls
directly is checked even if a test also defers it.

The rule is adapted from revive's `defer`, covering its recover,
immediate-recover, and return parts. The loop case belongs to `deferinloop`,
and the method-call and call-chain cases are not checked.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/defermisuse/cmd/defermisuse
go tool defermisuse ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -defermisuse ./...
go tool lesiw.io/tools/cmd/fix -defermisuse ./...
```
