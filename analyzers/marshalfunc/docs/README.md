# lesiw.io/tools/analyzers/marshalfunc

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/marshalfunc.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/marshalfunc)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to encoding/json that marshals a
value whose type contains a channel or a func.

```go
func f(c chan int) {
    json.Marshal(c) // marshaling a value whose type contains a channel
}
```

Marshal's documentation says "Channel, complex, and function values cannot be
encoded in JSON. Attempting to encode such a value causes Marshal to return an
UnsupportedTypeError." The rule covers channels and functions. Marshal
returns the error at run time when it encodes a channel or func.

Marshal, MarshalIndent, and Encoder.Encode use the same encoder, so all three
are checked. Struct fields are selected under the embedding and tag rules of
encoding/json; a slice or array's element, a map's value, and a pointer's
element are included. The message includes the path to the field or element
containing a channel or func.

The result depends on the static type alone. A nil pointer, a nil or empty
slice, and a nil or empty map with a supported key type encode without an
error and are reported too, as is a field omitted by omitempty or omitzero. A
value type parameter is reported when an exact term in its constraint contains
a channel or func; a tilde term is not reported, because a named type in the
set may implement a marshal method. A zero-length array is not reported, since
it has no element to encode.

A map is checked in two steps. When the key type is not always supported, an
exact key term that contains a channel or func is reported at `x[key]`; a
tilde term is left alone. If every key type is unsupported and none contains a
channel or func, the value is not checked, because the key is rejected first.
Otherwise the value is checked.

Not reported:

- an unexported field that is not an embedded struct, or a field tagged
  `json:"-"`; neither is encoded;
- a field hidden by an outer field, or by fields at the same depth and tag
  presence; neither is encoded;
- a value whose type implements `encoding/json.Marshaler` or
  `encoding.TextMarshaler`, because the method supplies the encoding instead of
  the fields; a map key is exempt only through `encoding.TextMarshaler`;
- an interface type, whose dynamic type is unknown;
- a map with an unsupported key type that does not contain a channel or func,
  because the key is rejected before the value is encoded;
- a call to another package's marshal function, including encoding/xml.

Methods with a pointer receiver apply to an addressable value, or to an outer
value that embeds a pointer to the method's type. A top-level value passed to
Marshal is not addressable, so a pointer receiver declared on its own type does
not exempt it; a field of a struct passed by pointer is addressable, so the
method does exempt it.

Report only: the repair is to change the type or the data, and neither follows
from the call.

The rule is like staticcheck's SA1026, narrowed to encoding/json and to
channels and functions.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/marshalfunc/cmd/marshalfunc
go tool marshalfunc ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -marshalfunc ./...
```
