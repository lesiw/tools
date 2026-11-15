# lesiw.io/tools/analyzers/methodexprcall

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/methodexprcall.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/methodexprcall)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a method expression called with its
receiver.

```go
type item struct{ name string }

func (i item) getName() string { return i.name }

func f(i item) {
    item.getName(i) // method expression called with its receiver; use a method call
}
```

The check leaves four shapes alone:

- An untyped argument. The call converts a constant, an untyped `nil`, or a
  comparison result to the receiver's type, and the method call would have to
  write that conversion: `0.M()`, `nil.M()` and `(a == b).M()` do not compile.
  A shift over an untyped left operand and a builtin call over constants are
  left alone for the same reason: the type comes from the call, and the
  recorded argument shows only what the call made of it. So `M(1 << n)` and
  `M(min(3, 4))` are skipped, and so is `M(min(three, four))`, where the method
  call would compile.
- A call that expands a multiple-value expression, where the receiver arrives
  together with the arguments it fills. A method call has one receiver
  position.
- An argument whose type has no method of that name, or a different one. An
  unnamed type declares no method, though it can carry a promoted one, and a
  name that resolves to a field or to another type's method is not the method
  the expression names.
- A method expression that is not the callee of a call, whether it is bound to
  a variable (`f := item.getName; f(i)`) or passed to a function.

A method expression on an interface takes any argument that implements it,
since both forms dispatch on the dynamic type. A call that pairs with a method
value passed elsewhere (`g := item.getName` beside `item.getName(i)`) is still
reported: the method value is not rewritten, and only the call changes. A typed
constant is reported, `T.M(T(3))` becoming `T(3).M()`.

The rule is adapted from go-critic's methodExprCall, which reports only.

Fixable. The result is the method call: the receiver, then the method name and
the arguments as written. Parentheses go around a receiver that is not a
primary expression, and around a receiver whose first token is a composite
literal in an `if`, `for`, `range` or `switch` header with no bracket still
around it, since its brace would read as the start of the block. An address
operator or a dereference is dropped where the operand can be called as a
method on its own: `(*T).M(&x)` becomes `x.M()`, and `T.M(*p)` becomes `p.M()`,
while `(*T).M(&T{})` keeps its operator because a pointer-receiver method call
needs an addressable value, and a composite literal is not one. A comment in
the text the edit deletes reports the call without a fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/methodexprcall/cmd/methodexprcall
go tool methodexprcall ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -methodexprcall ./...
go tool lesiw.io/tools/cmd/fix -methodexprcall ./...
```
