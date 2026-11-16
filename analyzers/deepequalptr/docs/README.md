# lesiw.io/tools/analyzers/deepequalptr

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deepequalptr.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deepequalptr)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `reflect.DeepEqual` call on a type with
an `Equal` method and a pointer field.

```go
type item struct {
    meta *int
}

func (i item) Equal(other item) bool {
    return i.meta == other.meta
}

func same(a, b item) bool {
    return reflect.DeepEqual(a, b) // reflect.DeepEqual does not call item.Equal; pointer field meta is followed to its pointee
}
```

The reflect package documentation says that pointer values are deeply equal
"if they are equal using Go's == operator or if they point to deeply equal
values" (https://pkg.go.dev/reflect#DeepEqual). `DeepEqual` does not call
`Equal`. It compares the fields, and at a pointer field it compares the values
the pointers point to. Two values can therefore compare equal through
`DeepEqual` while `Equal` reports them unequal.

The rule reports a call when both arguments have the same type, and that type,
after any pointers are followed, has an `Equal` method on the type or a pointer
to it. The method takes a parameter of its own receiver type and returns a
boolean type. The compared type must be a struct or a pointer to one. A pointer
field is read through struct fields, array and slice elements, and map values.
Pointers at the top of the compared type are followed, so `*T`, `**T`, and a
defined pointer to `T` read the same fields. The diagnostic names the path to
the first non-blank pointer field in declaration order. An array, a slice, or a
map field adds `[]` to the path for each container. A zero-length array field
is not read. A pointer field that is unexported and comes from another package
is named only as a pointer field.

Not reported:

- a type with no reachable pointer field;
- a type that is not a struct or a pointer to one;
- a type without an `Equal` method on the type or a pointer to it;
- a type whose `Equal` method takes another type;
- an `Equal` method that does not return a boolean type;
- a promoted `Equal` method;
- a same-package `Equal` method that dereferences the first direct pointer
  field;
- a field whose type is a zero-length array;
- a call whose arguments have different static types; the dynamic type behind
  an interface is not known;
- a value whose static type is an interface, a slice, a map, an array, a
  channel, or a type parameter;
- a field whose type is a channel, a function, an interface, or a type
  parameter, even when a type parameter's constraint admits only pointer types;
- a field named `_`;
- a call through a function value; only a direct call is matched;
- a test file; the rule skips test files.

An `Equal` method that reads the same pointee as `DeepEqual` is still reported
when the method is in another package; the rule checks only the method
signature there.

Report only: the repair calls `Equal` with the dereferences or addresses the
receiver requires, and changes behavior. Bind reflect.DeepEqual to a variable
when the structural comparison is intended.

The rule is inspired by staticcheck's SA1021, which reports a net.IP
comparison by its bytes instead of net.IP.Equal.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deepequalptr/cmd/deepequalptr
go tool deepequalptr ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deepequalptr ./...
```
