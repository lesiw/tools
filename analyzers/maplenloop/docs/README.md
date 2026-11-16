# lesiw.io/tools/analyzers/maplenloop

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/maplenloop.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/maplenloop)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to `len` on the map a range
statement iterates.

```go
func f(m map[string]int) {
    for k := range m {
        use(k, len(m)) // map length does not depend on the loop; evaluate it once before the loop
    }
}
```

The spec's section on [Map types](https://go.dev/ref/spec#Map_types) says the
number of map elements is the map's length, and that the length "may change
during execution", through an assignment to an element, a `delete`, or a
`clear`. The [range clause](https://go.dev/ref/spec#For_range) section says the
clause iterates through all entries of a map. A loop body that leaves the map
alone therefore reads the same length at every pass.

The map expression is a name, or a path of fields that goes through no pointer.
The map is one the function built with `make` or a literal, or a parameter, a
named result, or a receiver of the function. The loop must leave the map alone:
no assignment to the map, to one of its elements, or to a field of the value
that holds it, and no second name for it.

The call is also left alone in these cases:

- a map declared at package level, or a local copy of one, since another
  function may change it;
- an assignment of a map of its type, or of an element or a field of one, where
  the function did not build that map;
- a call that hands over a value of the map's type, a value that holds one, a
  value that names the map, or an interface parameter the caller can fill,
  anywhere in the function, since the callee can keep it;
- a call of a function value, whether a variable or a field, which may be a
  closure that captured the map;
- a send of a value that can carry the map on a channel;
- an assignment, anywhere in the function, that puts the map, a value that
  holds it, or a pointer to it into a name that has room for it, unless the
  value is a read of one element;
- the call is inside a function literal in the loop, which can outlive it;
- a `goto` after the loop that returns to it, which runs the loop again;

The fix declares the length above the loop as `length`, or the next free
`lengthN` when the name is already visible at a call, when an earlier fix took
it in the same function, or when the block declares it after the loop.

The fix is withheld when a comment lies inside the call, when a standalone
comment ends on the loop's line or the line above it, when the loop does not
begin its line, where a declaration cannot be written, when the call spans
lines, and when a `goto` from before the loop jumps over it, which Go rejects
for a declaration. The line width limit withholds a fix whose declaration, or
the line the name lands on, would exceed it. A reported call can therefore
arrive without a fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/maplenloop/cmd/maplenloop
go tool maplenloop ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -maplenloop ./...
go tool lesiw.io/tools/cmd/fix -maplenloop ./...
```
