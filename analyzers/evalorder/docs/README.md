# lesiw.io/tools/analyzers/evalorder

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/evalorder.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/evalorder)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a return that reads a value and passes its
address to a call.

```go
func add(p *int) int { *p++; return *p }

func incremented(n int) (int, int) {
	return n, add(&n) // n is read before or after a call that may write it
}
```

The "Order of evaluation" section of the Go spec fixes the order of function
calls, method calls, receive operations, and binary logical operations while
the operands of an expression, an assignment, or a return statement are
evaluated. It leaves the rest of the order unspecified, so `n` may be read
before `add` is called or after it returns.

A read is reported unless the order is known, in either direction. Calls run
left to right, so `same(n), add(&n)` reports nothing. A read inside the
arguments of a call that comes later is still reported, because it may be
evaluated before the write: `add(&n), same(n)` is a report. A callee writes
only when it runs, after every one of its arguments has been read, so
`both(&n, n)` reports nothing. The operands of `&&` and `||` run left to right,
so `add(&n) > 0 && same(n) > 0` reports nothing either.

Beyond the order the spec fixes, the read has to be one that a write through
the address can change. A read of a pointer is not a read of the data it names,
and a read of a slice header is not a read of an element. A write to `e.size`
cannot change `e.name`, so that pair reports nothing; `a[0]` and `a[1]` name
different storage, while an index that is not a constant matches every element.
A read of a type parameter is left alone when every type it can hold refers to
data that lives elsewhere.

An address counts only when a call receives it as a pointer parameter. A
function that takes an interface usually reads the value behind the address, so
an address passed as an interface is skipped: `fmt.Sprint(&n)`. A builtin is
skipped, because the signature the type checker writes for `append(ps, &n)`
makes the address look like a pointer argument. An address the return hands
back to its caller, as in `return n, &n`, reaches no callee. Neither does one
that a conversion wraps, as in `return n, (*int)(&n)`. Wrapped in a call, as in
`use((*int)(&n))`, it reports. An address that a variable holds, as in
`p := &n; use(p)`, is not followed.

A pointer parameter is assumed to write, because a writer is the case worth
catching. The receiver of a method call is assumed to read, because
`(*bytes.Buffer).String` and `(*sync.Map).Load` are common and write nothing. A
method value whose receiver is already a pointer copies the pointer, which is a
read. A method expression passes the receiver as an argument and does report:
`(*tally).bump(&t)`.

An array slice such as `a[:]` takes the array's address and reads nothing, so
it reports nothing; a string slice reads the string and reports. A variadic
parameter counts when its element type is a pointer. An argument that carries
no address is not looked at, so `return s[0], fill(s)` reports nothing even
though `fill` may write `s[0]`. Only the operands of a return are checked; an
assignment carries the same order dependence and is not looked at.

A function literal's body runs where it is called and not where it is written.
An address the body hands out while the literal runs counts, and a return
inside one is a return of its own, reported like any other.

A variable is reported once, however many results read it.

The rule is adapted from go-critic's evalOrder: this one reports a return only,
counts pointer parameters rather than pointer methods, and follows a field, an
element, and a dereference back to the variable they start at.

Report only: hoisting the call and copying the value each settle the order, and
the tool cannot tell which value was meant.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/evalorder/cmd/evalorder
go tool evalorder ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -evalorder ./...
```
