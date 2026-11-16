# lesiw.io/tools/analyzers/badcall

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/badcall.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/badcall)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call that a constant or nil argument
leaves with nothing to do.

```go
func f(s string, h hash.Hash) {
    _ = strings.Replace(s, "a", "b", 0)      // a count of zero replaces nothing
    _ = strings.EqualFold("alpha", "alphas") // the arguments differ in length, so the comparison is always false
    h.Write(nil)                             // a nil buffer adds nothing to the hash
}
```

`strings.Replace` replaces "the first n non-overlapping instances" of `old`,
and `bytes.Replace` takes the same count with the same meaning. A count of zero
replaces nothing, and only a constant count is examined. A negative count
replaces every instance, so it is left alone. `bytes.Replace` copies even when
the count is zero, and `bytes.Clone` makes the copy without the replace.

`strings.EqualFold` reports whether two strings "are equal under simple Unicode
case-folding", and `bytes.EqualFold` does the same for byte slices. ASCII
folding maps one byte to one byte, so two ASCII constants of different lengths
can never fold equal. `bytes.EqualFold` finds a constant through a conversion
to a byte slice, so `[]byte("alpha")` counts. Constants of equal length are
left alone: the rule reports a comparison that cannot match, not a constant
condition. A constant that is not ASCII is left alone too. Only in ASCII is a
byte length a rune count, and a fold can map a multi-byte rune to a one-byte
one. The Kelvin sign is three bytes and folds to `"k"`, which is one byte.

`hash.Hash.Write` "adds more data to the running hash". A nil buffer adds
nothing. Only a nil buffer is reported: one written as `nil`, or as a byte
slice converted from `nil`.

A `Write` the analyzer can see declared on a concrete type is left alone. The
`Write` must be declared by an interface, and the receiver's type, or a pointer
to it, must implement `hash.Hash`, so a hash assigned to an `io.Writer` is left
alone. A call through an interface is reported whatever its dynamic type, which
the analyzer cannot see, and a slice held in a variable is left alone, even
when it is nil.

A call through a function or method value held in a variable is not read, since
its callee is a variable. A method expression passes the receiver first, so its
call has two arguments and is left alone. A dot import and a parenthesized callee
are both examined.

The `Replace` check is adapted from go-critic's `badCall`, which reports the
zero count of the two `SplitN` functions too and reads a zero count as a
mistake for `-1`; staticcheck's SA1018 reports the count as well. This rule
examines the `Replace` count only: a zero count in `SplitN` returns an empty
slice, which Go's own examples write to print the nil result, where a zero
count in `Replace` always returns its input.

Report only: replacing the call with its input, or with `false`, would keep the
mistake. A write is left alone too, since the analyzer cannot see which `Write`
runs, and removing the call could remove work a wrapper does.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/badcall/cmd/badcall
go tool badcall ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -badcall ./...
```
