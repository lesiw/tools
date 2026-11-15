# lesiw.io/tools/analyzers/marshalrecv

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/marshalrecv.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/marshalrecv)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a marshal method that writes to a value
receiver.

```go
type item struct {
    text []byte
}

func (i item) MarshalText() ([]byte, error) {
    i.text = []byte("0") // writes to a copy of item; the caller never sees it
    return nil, nil
}
```

A method declared with a value receiver works on a copy of the value it was
called for, so a write to the receiver lands in that copy and the caller never
sees it. Two intents share the form: a store that later calls do not see, and a
mask or a normalization of the copy before it is encoded. Writing through a
local shows which: `local := i; local.text = nil`. The write a method reads
back is reported too, because the source does not say which of the two it is.

The methods reported are `MarshalJSON`, `MarshalText`, `MarshalBinary` and
`GobEncode`, matched with the signature they share, `() ([]byte, error)`.

A write is reported when the left side names the receiver, a field of it, or an
element of an array it holds, reached directly or through `&` applied to the
receiver. A slice of that array writes into the copy too, so a write through
one is reported. A pointer, a slice or a map the receiver holds leads to
storage the caller shares, so a write there is not reported. Calling a method
on the receiver, or passing its address to a function that writes through it,
is not reported either: a call is not followed.

The x/tools `unusedwrite` pass reports a field or an array element the method
never reads back. This reports the write either way, and it reports a write to
the receiver itself, which `unusedwrite` does not report.

Report only: naming the copy keeps the behavior, but the rewrite renames every
use of the receiver in the body and needs a fresh name, which x/tools does not
yet export a way to choose. Moving the method to a pointer receiver keeps the
write, and takes the value type out of the interface's method set.

The idea comes from revive's modifies-value-receiver, which reports the write in
any method; this narrows it to the standard library's four marshal methods.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/marshalrecv/cmd/marshalrecv
go tool marshalrecv ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -marshalrecv ./...
```
