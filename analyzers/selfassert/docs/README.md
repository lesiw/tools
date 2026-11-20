# lesiw.io/tools/analyzers/selfassert

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/selfassert.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/selfassert)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a type assertion from a type to itself.

```go
func f(r any) any {
    return r.(any) // type assertion from a type to itself
}
```

The spec says a type assertion "asserts that x is not nil and that the value
stored in x is of type T" (https://go.dev/ref/spec#Type_assertions). A non-nil
interface value always holds a type the interface allows. So asserting a
value's own type fails only when the value is nil, and the assertion is a nil
check written the long way.

The two-value form yields the value and whether the assertion held, so a nil
value makes it false:

```go
type filter interface {
    allow() bool
}

func load(f filter) bool {
    g, ok := f.(filter) // type assertion from a type to itself
    return ok && g.allow()
}
```

Not reported:

- an assertion from another type, as `r.(filter)` where `r` is `any`, which
  reads a narrower type out of the interface and can fail on a value that is
  not nil;
- an assertion to another interface the value's type already implements, as
  `f.(other)` or `f.(interface{ allow() bool })`. That assertion also fails
  only on nil, but the rule reads the identical type alone: the code that
  follows works with the type of the result;
- an assertion to an interface the value's type does not implement, as
  `f.(keen)`, which can fail on a value that is not nil;
- a type switch, as `switch x.(type)`, which names no destination type to
  compare;
- a conversion, as `any(f)`, which is not an assertion.

## The fix

Where one result is kept, the edit writes that one, so the assertion becomes
the nil check it stood for: `_, ok := f.(filter)` becomes `ok := f != nil`, and
`g, _ := f.(filter)` becomes `g := f`. The value is read once there, so a call
or a receive is no obstacle.

Where both results are kept, the fix writes the value and a comparison against
nil:

```go
func load(f filter) bool {
    g, ok := f, f != nil
    return ok && g.allow()
}
```

The assertion is reported without a fix in these cases:

- a call or a receive would be read twice, and the reads can disagree;
- a comment sits inside the text the edit replaces;
- the result does not fit on the line. A line already past the width is fixed
  anyway, since refusing cannot shorten it;
- a `var` declaration keeps one result, because the short declaration it would
  become is a different statement;
- the left side keeps neither result, so there is nothing to write.

The one-value form is reported without a fix too: the assertion is where a nil
interface panics, so deleting it changes what the program does.

## Provenance

The rule is adapted from staticcheck's `S1040`, which reports "Type assertion
to current type" and offers no fix, and advises writing a nil check out as a
comparison (https://staticcheck.dev/docs/checks/#S1040). go-critic's
`sloppyTypeAssert` reports the same shape.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/selfassert/cmd/selfassert
go tool selfassert ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -selfassert ./...
go tool lesiw.io/tools/cmd/fix -selfassert ./...
```
