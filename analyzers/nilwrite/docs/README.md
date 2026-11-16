# lesiw.io/tools/analyzers/nilwrite

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilwrite.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilwrite)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a Write method that may modify the byte
slice it was given.

```go
func (sink) Write(p []byte) (int, error) {
    p[0] = 'x' // Write may modify the buffer it was given
    return len(p), nil
}
```

io.Writer's doc says "Write must not modify the slice data, even temporarily."
A caller may pass a buffer it reuses, so a method that stores into p[0] or
appends to p may change data the caller still reads.

The rule reports a write into the parameter of a method named Write whose
signature is io.Writer's. It reports an index assignment, including a compound
assignment or an increment, and an append, copy, or clear whose destination is
the parameter or a slice of it.

The rule leaves alone:

- a method whose signature differs from io.Writer's in its parameter count,
  parameter type, result type, or variadic form;
- a write through another name for the same slice, as q := p; q[0] = 0; the
  rule does not follow other variables, so this write reaches p and is missed;
- a write inside a function that receives p, since the rule does not follow
  the value into another body;
- a write to a local variable that shadows p;
- an append whose first argument is a three-index slice whose high and max
  bounds are equal constants or the same side-effect-free expression, since
  the slice has no spare capacity and append allocates;
- a copy or clear whose destination is a slice expression with a constant
  zero length, as copy(p[:0], src) or clear(p[:0:0]);
- a copy whose source is a typed nil slice, an empty string, an empty slice
  literal, or a slice expression with a constant zero length, as
  copy(p, []byte(nil)), copy(p, ""), copy(p, []byte{}), or copy(p, src[:0]);
- an append with no values, as append(p);
- the rebinding statement itself, as p = nil or p = p[:0]; a write after p is
  reassigned is still reported.

A copy or clear into the parameter is reported when its destination length is
not a constant, even if that length is zero at run time; a copy with an empty
source is exempt as above.

Report only: copying p changes where the bytes go, and writing into p is the
fault, so the repair is the author's.

The rule is adapted from staticcheck's SA1023, which reports a store to an
element of p and an append to p.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilwrite/cmd/nilwrite
go tool nilwrite ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilwrite ./...
go tool lesiw.io/tools/cmd/fix -nilwrite ./...
```
