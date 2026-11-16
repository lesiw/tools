# lesiw.io/tools/analyzers/deepequalptr2

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deepequalptr2.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deepequalptr2)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `reflect.DeepEqual` call on two pointers,
including `unsafe.Pointer` values.

```go
func same(left, right *int) bool {
    return reflect.DeepEqual(left, right) // reflect.DeepEqual on two pointers compares what they point to; compare the pointers
}
```

The reflect package documentation says two pointer values are deeply equal "if
they are equal using Go's == operator or if they point to deeply equal values"
(https://pkg.go.dev/reflect#DeepEqual). Under the second clause, `DeepEqual`
returns true for two distinct pointers of the same type that point to deeply
equal values. To test pointer identity, use ==.

The rule reports a direct call to `reflect.DeepEqual` in three cases: both
arguments have the same pointer type and a plain pointee, both arguments have
different pointer types that are assignable, or both arguments have the same
`unsafe.Pointer` type.

A pointee is plain when its underlying type is a basic type (including
`unsafe.Pointer`), a channel, a zero-length array, or an array or struct of
plain types. In the same-type case, the rule leaves the call alone when the
pointee is uncomparable, except when every uncomparable part is inside a
zero-length array; when the pointee's underlying type is a pointer (other than
`unsafe.Pointer`) or an interface, or a struct or array whose compared elements
contain either, unless the pointer or interface is inside a zero-length array;
when the pointee is a
type parameter, or a struct or array whose compared elements contain one,
unless the type parameter is inside a zero-length array; or when the pointee
has an `Equal` method whose only parameter is the pointee type or a pointer to
it and whose only result is a boolean. The method is the author's equality
test, and the pointer comparison would not call it. A slice, map, or function
has no == comparison, so `DeepEqual` compares the pointees with its own rules.

An argument of type parameter or interface type is not a pointer, and neither
is an untyped nil, so a call with any of them is left alone. When the pointer
types differ and one is assignable to the other, `DeepEqual` is always false,
so the rule reports the call whatever the pointee.
When both arguments have the same `unsafe.Pointer` type, the call is reported;
`DeepEqual` compares the values with ==, so the fix changes nothing.

Not reported for two arguments of the same pointer type:

- a pointer whose pointee is uncomparable, except when every uncomparable part
  is inside a zero-length array, as with `*[]int`, `*map[int]int`, `*func()`,
  or a struct with a field of one of those types;
- a pointer whose pointee is a pointer (other than `unsafe.Pointer`) or an
  interface, or a struct or array whose compared elements contain either,
  unless the pointer or interface is inside a zero-length array, as with
  `*any`, `**int`, or a struct with a pointer field;
- a pointer whose pointee has an `Equal` method whose only parameter is the
  pointee type or a pointer to it and whose only result is a boolean;
- a pointer whose pointee is a type parameter, or a struct or array whose
  compared elements contain one, unless the type parameter is inside a
  zero-length array.

Not reported in every case:

- an argument of type parameter or interface type, or an untyped nil; none is
  a pointer;
- arguments that are not assignable to each other, as with two defined
  pointer types with the same underlying type;
- an argument whose underlying type is neither a pointer nor
  `unsafe.Pointer`, as with an `int`, a slice, a map, a channel, or a
  function;
- a call through a function value; only a direct call to `reflect.DeepEqual`
  is matched;
- a test file.

The fix replaces the call with the pointer comparison. For two pointers of the
same type, it changes the answer for distinct pointers whose pointees are
deeply equal. For two assignable pointer types that are not identical, it
changes the answer for the same pointer as well. For the same `unsafe.Pointer`
type, it changes nothing. To keep the comparison of the pointees, write
`*left == *right` where neither pointer can be nil and the pointee type is
comparable; otherwise bind `reflect.DeepEqual` to a variable. The fix is
withheld where a comment lies in
the text it would replace, where the call or the printed form of an argument
spans lines, and where the call is the expression of an expression statement
or the call of a `go` or `defer` statement. The fix parenthesizes the
comparison under a unary operator, as an operand of == or !=, and in an `if`,
`for`, or `switch` header when an argument contains a composite literal. The
suggested fix message notes when the edit changes behavior.

The rule is adapted from staticcheck's SA1021, which reports a net.IP
comparison by its bytes.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deepequalptr2/cmd/deepequalptr2
go tool deepequalptr2 ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deepequalptr2 ./...
go tool lesiw.io/tools/cmd/fix -deepequalptr2 ./...
```
