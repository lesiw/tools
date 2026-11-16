# lesiw.io/tools/analyzers/headercanon

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/headercanon.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/headercanon)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a non-canonical key in an `http.Header`
map.

```go
func f(h http.Header) {
    _ = h["content-type"]   // header key "content-type" is not canonical
    h["x-request-id"] = nil // header key "x-request-id" is not canonical
}
```

The net/http `Header` doc says the keys "should be in canonical form, as
returned by CanonicalHeaderKey". `Set` says "To use non-canonical keys, assign
to the map directly", and `Get` and `Values` say the same about access. A key
in a variable is left alone, so direct map access remains available. `Set`,
`Add`, `Get`, `Values`, and `Del` canonicalize their key, so a direct constant
key selects a different entry, and a later `Get` canonicalizes the key and
returns the zero value.

The rule reads an index expression or a delete call on a value of type
`http.Header`, through any alias, and through a type parameter whose type set
is only that type. It reports both reads and writes. A map typed
`map[string][]string` is left alone: the type is not `http.Header`.

The key is a string constant. An empty key is already canonical. A key with a
space, or any invalid header field byte, is left alone, because
`http.CanonicalHeaderKey` returns it "without modifications".

The fix applies to a direct key. The fix replaces a string literal with its
canonical form, and wraps any other constant in `http.CanonicalHeaderKey`
while keeping its expression: `versionKey` becomes
`http.CanonicalHeaderKey(versionKey)`. A parenthesized literal that carries a
comment is edited in place: only the literal changes. The fix writes the
qualifier the file binds `net/http` to, an unqualified `CanonicalHeaderKey`
under a dot import or inside net/http, and adds `net/http` when the file has
no import for it. For a nonliteral key, it is withheld when the canonicalizer
cannot be named at the key's position, as when a local `http` shadows the
package or the import is blank.

A composite literal key is not reported: the key may be deliberate, as in a
test. A direct key that appears noncanonically in a header-shaped composite
literal in the package is report-only, and the report stays because a method
such as `Set` or `Get` still canonicalizes the key and misses that entry. A
package that has another direct key the rule does not report and cannot prove
canonical also gets no fixes, since that key may be the same entry. That
includes a key in a variable and a noncanonical key on a map of type
`map[string][]string`, `textproto.MIMEHeader`, or a defined equivalent. A
nonconstant key the rule cannot prove canonical in a header-shaped composite
literal withholds every fix in the package too. The rule treats a call to
`http.CanonicalHeaderKey` or `textproto.CanonicalMIMEHeaderKey` as canonical,
so it does not withhold a fix. The rule does not examine an access in another
package.

The rule is adapted from staticcheck's `SA1008`.

Fixable, except for a direct key that appears noncanonically in a header-shaped
composite literal, a package with another direct key the rule does not report
and cannot prove canonical, a nonconstant key the rule cannot prove canonical
in a header-shaped composite literal, or a nonliteral key whose canonicalizer
cannot be named at its position.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/headercanon/cmd/headercanon
go tool headercanon ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -headercanon ./...
go tool lesiw.io/tools/cmd/fix -headercanon ./...
```
