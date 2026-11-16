# lesiw.io/tools/analyzers/jsonrecv

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/jsonrecv.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/jsonrecv)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports `MarshalJSON` on a pointer receiver that
does not need the pointer and `UnmarshalJSON` on a value receiver that writes
directly to its copy.

```go
func (i *item) MarshalJSON() ([]byte, error) { // MarshalJSON has a pointer receiver; a value of the type does not implement json.Marshaler
    return nil, nil
}

func (u user) UnmarshalJSON(b []byte) error { // UnmarshalJSON has a value receiver; its writes land in a copy
    u.label = string(b)
    return nil
}
```

The spec's Method sets section says the method set of `T` holds the methods
declared with receiver `T`, and the method set of `*T` also holds those
declared with receiver `*T`. A pointer receiver is therefore in the method set
of `*T` only, and a value of `T` does not implement `json.Marshaler`.

The v1 `Marshal` in `encoding/json` takes the address of an addressable value
and calls the method on that address. A value that is not addressable, such as
a map element or a value held in an interface, is encoded without it. The
`encoding/json/v2` documentation says that `Marshal` "ensures that a value is
always addressable (by copying the value if necessary)", so it calls the method
either way.

A value receiver works on a copy; Effective Go's "Pointers vs. Values" section
says that a call on a value "would cause the method to receive a copy of the
value, so any modifications would be discarded." A write to the receiver or to
a field of it lands in that copy, and the value `encoding/json` decodes into
keeps its old fields. The write is lost to the caller even when the method
reads it back. A method that only reads its input is not reported. A write made
through a call or a local alias is not followed, so a value receiver that
writes in one is left alone. A write through a pointer, slice or map the
receiver holds is not reported either.

A pointer receiver is left alone when the method writes to the receiver or a
field of it, or compares it with the nil identifier using `==` or `!=`, because
a value receiver would lose the write and cannot test for a nil receiver. A
write through a pointer, slice or map the receiver holds reaches shared
storage, so a value receiver would keep it and the method is still reported. A
pointer receiver is also left alone when the receiver's type contains a field
of a synchronization type, including an array of one, because a value receiver
would copy it. A defined type over a synchronization type is not recognized. A
call through a pointer-receiver method on the receiver or the address of a
field of it, or one that receives the receiver or a slice of an array in it, is
treated as a write. A call through a pointer field or an embedded pointer, a
call that receives the pointer receiver's address, and a call that reaches the
value another way are not followed. A local alias is not followed. The report
is a convention: a value of the type does not implement `json.Marshaler`, and a
type that is only used through pointers still encodes correctly. A pointer
receiver on a type that is only used through pointers, or on a type that is
expensive to copy, is still reported; `//ignore` on the method records that
choice.

A method is reported only when its signature matches the interface method it is
named for: `() ([]byte, error)` for `MarshalJSON`, and `([]byte) error` for
`UnmarshalJSON`. A method with another signature does not implement the
interface, so `encoding/json` does not call it. `MarshalText`, `MarshalYAML`
and their unmarshal counterparts are left alone.

Report only. For `MarshalJSON`, a value receiver copies the receiver on every
call; for `UnmarshalJSON`, a pointer receiver takes the method out of `T`'s
method set, so a caller that holds a `T` in an interface may stop satisfying
`json.Unmarshaler`. Neither change is safe without knowing how the method uses
the receiver.

A write to a field of a value receiver may also carry a `valuereceiver` report.

The rule is adapted from revive's `marshal-receiver`, which reports every
`MarshalJSON`, `MarshalText`, `MarshalYAML` and unmarshal counterpart by
receiver kind alone. This narrows it to JSON, to `MarshalJSON` on a pointer
receiver, and to an `UnmarshalJSON` value receiver that writes to its copy.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/jsonrecv/cmd/jsonrecv
go tool jsonrecv ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -jsonrecv ./...
```
