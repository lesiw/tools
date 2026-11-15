# lesiw.io/tools/analyzers/underef

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/underef.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/underef)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a redundant dereference in front of a
selector.

```go
type item struct{ Kind, Name string }

func kind(p *item) string {
    return (*p).Kind // redundant dereference in front of a selector
}
```

The Selectors section of the spec says that for a value x of type T or *T,
where T is not a pointer or interface type, x.f denotes the field or method at
the shallowest depth in T. It also says that for a defined pointer type x, x.f
is shorthand for (*x).f when f is a field. A field or a value method selected
through a nil pointer panics with or without the dereference.

The dereference stays in front of a method whose receiver type is the type of
`p`. There the dereference is the nil check: `(*p).f()` panics on a nil p, and
`p.f()` calls f with a nil receiver. The method value of that same method keeps
it too: `(*p).f` panics when the method value is evaluated, and `p.f` binds a
nil receiver and panics only when the body dereferences it.

A type parameter keeps it as well, since its methods come from the constraint:
`(*p).f()` panics on a nil p, and `p.f()` can reach a pointer-receiver method
the constraint supplies. A field cannot be selected through a type parameter at
all, so `(*p).Kind` keeps its dereference there too.

A method promoted from an embedded field is reported: reading the embedded
field dereferences the operand either way, so the nil behavior does not change.

In each of these shapes the shorthand does not compile:

- a method selected through a defined pointer type, where the shorthand covers
  a field only.
- a pointer to a pointer, and a pointer to a defined pointer type.
- a pointer to an interface.
- a pointer to a type parameter.

A method expression is left alone: its operand is a type rather than a value,
and `(*T).M` takes a `*T` where `T.M` takes a `T`.

The rule is adapted from go-critic's underef, which also reports an index
through a pointer to an array.

Fixable: the fix drops the `*` and the parentheses around it. The parentheses
stay when the operand is a dereference or another unary expression; the
selector would bind to the operand first. A line break inside the parentheses
withholds the fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/underef/cmd/underef
go tool underef ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -underef ./...
go tool lesiw.io/tools/cmd/fix -underef ./...
```
