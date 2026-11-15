# lesiw.io/tools/analyzers/valuereceiver

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/valuereceiver.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/valuereceiver)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a dead write to a field of a value
receiver.

```go
func (p pair) Set(left int) {
    p.left = left // dead write to field left of a value receiver
}
```

A value receiver works on a copy, so a write to its field is dead when nothing
reads the field after it.

`p.left++` and the target of a range clause, `for p.left = range m`, are
writes too. The reads a write makes of the field do not count: the statement
doing the writing, and the address of any assignment target.

A use of the receiver as a value saves the writes before it, whether the value
is returned, passed on, used as a map key, called as a method, or assigned. A
read anywhere else in a loop that contains the write saves it, whether or not
the loop runs again. A read in any func literal in the method also saves every
write to that field, even one that nothing calls, because the analyzer cannot
tell which literals are called. The analyzer compares positions, so a read on a
branch that cannot follow the write still saves it.

A nested field is read and written by its path, so a write to `in.left` is dead
even when the method reads `in.right`. A promoted field is reported by the path
it is embedded in, so `e.left = left` reports `pair.left`.

`unusedwrite`, already part of this suite, reports the plain dead store from
SSA stores. A plain store to a direct field therefore carries an `unused write
to field left` diagnostic from `unusedwrite` and a `dead write to field left of
a value receiver` diagnostic from this rule. Every other shape this rule
reports carries only its own: an increment, a compound assignment, a write
followed by a second write, a write whose field is read only before it, a
nested field, and a write inside a func literal.

Not reported, and why:

- a write in a method that hands out the copy's address, by an `&` of the copy
  or of one of its fields, by a slice of an array it holds, or by a call to a
  method with a pointer receiver: the pointer carries the write to code whose
  reads the analyzer cannot place;
- a write in a method that contains a `goto`, because the jump can carry a read
  backwards;
- a write through a pointer field, a map or slice the receiver shares, or a
  dereference, all of which reach the caller's own data;
- a write to the receiver itself, `p = pair{}`, which is not a field;
- an element write to an array, because an index is not a field. Neither
  analyzer reports an element write on an array field; `unusedwrite` reports
  one on an array receiver.

The idea comes from revive's modifies-value-receiver.

Report only: the repair an author wants is a pointer receiver, which changes
behavior by sending the write to the caller's value, so a tool cannot apply it.
It also moves the method out of the type's method set, where a caller holding
the value in an interface would stop compiling. A type that also has pointer
methods is reported by `recvtype`, and making the receivers pointers answers
both.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/valuereceiver/cmd/valuereceiver
go tool valuereceiver ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -valuereceiver ./...
```
