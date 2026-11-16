# lesiw.io/tools/analyzers/encodeoverlap

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/encodeoverlap.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/encodeoverlap)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an encoder called with a destination and a
source that start at the same index of one backing array.

```go
func encode(dst []byte) {
    base64.StdEncoding.Encode(dst, dst) // dst and src share memory
}
```

encoding/base64's `Encoding.Encode` documentation says it "encodes src using
the encoding enc" and that it writes `Encoding.EncodedLen(len(src))` bytes to
dst. It does not say what an overlap does. The encoder reads three source bytes
and writes four destination bytes, so a destination that starts where the
source starts can overwrite source bytes before they are read. The other
encoders write to the destination before they read the next source bytes too;
their `EncodedLen` and `MaxEncodedLen` methods and functions return an output
length or an upper bound
(https://pkg.go.dev/encoding/base32#Encoding.EncodedLen,
https://pkg.go.dev/encoding/hex#EncodedLen,
https://pkg.go.dev/encoding/ascii85#MaxEncodedLen).

The rule covers `Encoding.Encode` in `encoding/base32` and `encoding/base64`,
and `Encode` in `encoding/hex` and `encoding/ascii85`. It reports a call when
both arguments are slices of one backing array that start at the same index.
An assignment to a local name, a conversion that keeps the backing array, and
a branch that takes the same value on every path carry the value to the call.
Two reads of the same variable or field in one block count as the same slice
when no call, write, or send lies between them. A call through a bound method
value is followed when the method value is a local variable used directly. An
omitted lower bound counts as zero, and two constant bounds are compared by
value.

Not reported:

- a nil constant;
- a destination and a source from different arrays;
- a source with a visible zero length, such as `buf[:0]`, which reads no
  bytes;
- an argument converted from a string, because each conversion allocates a new
  array;
- `AppendEncode`; the rule covers `Encode` and does not model an append;
- a function named `Encode` from another package or a local declaration;
- a bound method value read from memory, such as a package variable, a field,
  or an address-taken local;
- an encoder called through an interface value.

The rule has these limits:

- two slices of one backing array with different lower bounds are not
  compared, even when their ranges overlap;
- a slice of a slice is resolved only when the immediate base is the same
  value, so two separately built inner slices are not compared;
- a branch whose paths produce different slice values is not followed, even
  when the values start at the same index;
- a copy held in an address-taken local is not followed;
- two reads count only in one block with no call, write, or send between them,
  so a value written through a pointer or field between reads is not connected;
- a type parameter whose type set is a slice is not recognized;
- a zero length read from memory is not recognized.

Report only: a repair copies the source into a separate buffer before the call,
and the rule cannot tell whether the added allocation is acceptable.

The rule is inspired by staticcheck's SA1031.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/encodeoverlap/cmd/encodeoverlap
go tool encodeoverlap ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -encodeoverlap ./...
```
