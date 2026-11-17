# lesiw.io/tools/analyzers/deepequalptr

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deepequalptr.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deepequalptr)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `reflect.DeepEqual` call on a type with an
`Equal` method and a pointer field, or on two pointers.

The reflect package documentation says that pointer values are deeply equal
"if they are equal using Go's == operator or if they point to deeply equal
values" (https://pkg.go.dev/reflect#DeepEqual). `DeepEqual` compares the value
behind a pointer, returns false when the two dynamic types differ, and never
calls an `Equal` method. The rule has two arms, one for an `Equal` method and
one for `==` on the pointers themselves.

Both arms skip a test file, and only a direct call to `reflect.DeepEqual` is
matched.

## The Equal arm

```go
type item struct {
    meta *int
}

func (i item) Equal(other item) bool {
    return i.meta == other.meta
}

func same(left, right item) bool {
    return reflect.DeepEqual(left, right) // reflect.DeepEqual does not call item.Equal; pointer field meta is followed to its pointee
}
```

The arm reports a call whose arguments have the same type. That type is a
struct or a pointer to one. It has an `Equal` method whose parameter is its own
receiver type and whose result is a boolean; the receiver may be the type or a
pointer to it. A pointer field must be reachable from the type. A pointer field
is read through struct fields, array and slice elements, and map values, and
the path adds `[]` for each container. Pointers at the top of the compared type
are followed, so `*T`, `**T`, and a defined pointer to `T` read the same fields.
The message includes the path to the first non-blank pointer field in
declaration order. A zero-length array field is not read. A pointer field that
is unexported and comes from another package is named only as a pointer field.

Not reported by the Equal arm:

- a type with no reachable pointer field;
- a type that is not a struct or a pointer to one;
- a type without an `Equal` method on the type or a pointer to it;
- a type whose `Equal` method does not take exactly one parameter of its own
  receiver type;
- a type whose `Equal` method does not return exactly one boolean;
- a promoted `Equal` method;
- a call whose same-package `Equal` method dereferences the first direct
  pointer field;
- a field whose type is a zero-length array;
- a call whose arguments have different static types; the dynamic type behind
  an interface is not known;
- a field whose type is a channel, a function, an interface, or a type
  parameter, even when a type parameter's constraint admits only pointer types;
- a field named `_`.

An `Equal` method that reads the same pointee as `DeepEqual` is still reported
when the method is in another package; the arm checks only the method signature
there.

## The identity arm

```go
func samePointer(left, right *int) bool {
    return reflect.DeepEqual(left, right) // reflect.DeepEqual on two pointers compares what they point to; compare the pointers
}
```

The arm reports a direct call whose two arguments are the same pointer type
with a plain pointee, two assignable pointer types that are not identical, or
two values of the same `unsafe.Pointer` type.

A pointee is plain when its underlying type is a basic type (including
`unsafe.Pointer`), a channel, a zero-length array, or an array or struct of
plain types. A slice, map, or function has no `==` comparison, so a pointee of
one of those types is not plain. Every field counts, including one named `_`.
A type with a reachable pointer field is not plain, so the identity arm leaves
its calls alone; the Equal arm reports the call when the `Equal` method takes
its own receiver type.

Not reported by the identity arm for two arguments of the same pointer type:

- a pointer whose pointee is uncomparable, as with `*[]int`, `*map[int]int`,
  `*func()`, or a struct with a field of one of those types, except when every
  uncomparable part is inside a zero-length array;
- a pointer whose pointee is a pointer (other than `unsafe.Pointer`) or an
  interface, or a struct or array with a field or element of one of those
  types, as with `*any`, `**int`, or a struct with a pointer field, unless the
  pointer or interface is inside a zero-length array;
- a pointer whose pointee has an `Equal` method whose only parameter is the
  pointee type or a pointer to it and whose only result is a boolean; the
  method is the author's equality test, and `==` does not call it;
- a pointer whose pointee is a type parameter, or a struct or array with a
  field or element of one, unless the type parameter is inside a zero-length
  array.

Not reported by the identity arm whatever the argument types are:

- an argument of type parameter or interface type, or an untyped nil; none is
  a pointer;
- arguments that are not assignable to each other, as with two defined pointer
  types with the same underlying type;
- an argument whose underlying type is neither a pointer nor `unsafe.Pointer`,
  as with an `int`, a slice, a map, a channel, or a function.

## The fix

The identity arm has a `SuggestedFix` that replaces the call with the pointer
comparison. For two pointers of the same type, it changes the answer for
distinct pointers whose pointees are deeply equal. For two assignable pointer
types that are not identical, it also changes the answer when both arguments
hold the same pointer. For the same `unsafe.Pointer` type, it changes nothing.
To keep the comparison of the pointees, write `*left == *right` where neither
pointer can be nil and the pointee type is comparable; otherwise bind
`reflect.DeepEqual` to a variable. The fix is withheld where a comment lies in
the text it would replace, where the call or the printed form of an argument
spans lines, and where the call is the expression of an expression statement or
the call of a `go` or `defer` statement. It parenthesizes the comparison under
a unary operator, as an operand of `==` or `!=`, and in an `if`, `for`, or
`switch` header when an argument contains a composite literal. The suggested fix
message notes when the edit changes behavior.

The Equal arm reports only. The repair writes a call to the method, whose
result the analyzer does not compute, and the receiver's form decides whether
an argument is a value, an address, or a dereference. Bind
`reflect.DeepEqual` to a variable when the structural comparison is meant.

## Provenance

The Equal arm is inspired by staticcheck's SA1021, which reports a net.IP
comparison by its bytes instead of net.IP.Equal. The identity arm is adapted
from the same rule, applied to any pointer rather than to net.IP.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deepequalptr/cmd/deepequalptr
go tool deepequalptr ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deepequalptr ./...
go tool lesiw.io/tools/cmd/fix -deepequalptr ./...
```
