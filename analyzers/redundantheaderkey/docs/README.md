# lesiw.io/tools/analyzers/redundantheaderkey

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/redundantheaderkey.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/redundantheaderkey)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to `http.CanonicalHeaderKey` on
the key argument of an `http.Header` method.

```go
func f(h http.Header) {
    h.Get(http.CanonicalHeaderKey("some-key"))
}
```

The net/http `Header` doc says the keys "should be in canonical form, as
returned by `CanonicalHeaderKey`". The docs for `Add`, `Set`, `Get`, `Del`,
and `Values` on that page say the method canonicalizes the key
(https://pkg.go.dev/net/http#Header.Add), so the call is unnecessary.

The call is reported when it is the key argument of a direct call to one of
those methods. This includes a method expression such as
`http.Header.Get(h, http.CanonicalHeaderKey(key))` and a method promoted
through an embedded `http.Header`. A `CanonicalHeaderKey` call on a value
argument is left alone, as in
`h.Set(key, http.CanonicalHeaderKey(value))`. A `CanonicalHeaderKey` used for
direct map access is left alone because direct map access does not
canonicalize the key. The rule covers `http.Header` only, so a method on
another type, such as `textproto.MIMEHeader.Get`, is out of scope. A method
value called through a variable, as in
`get := h.Get; get(http.CanonicalHeaderKey(key))`, is left alone, as is a
`CanonicalHeaderKey` held in a function value, as in
`chk := http.CanonicalHeaderKey; h.Get(chk(key))`.

The fix replaces the call with its argument, so
`h.Get(http.CanonicalHeaderKey(key))` becomes `h.Get(key)`. The fix is
withheld when a comment lies in the text it would replace, when the fixes
together would leave a dot import of `net/http` unused, or when the key spans
more than one line. A normal import is removed when its last use goes, but a
dot import is not, so the fix is withheld instead of leaving the file unable
to load. A nested call may also be reported without a fix so two suggested
fixes do not overlap. The diagnostic is still reported.

The rule is adapted from staticcheck's S1035, which reports the same call for
`Add`, `Del`, `Get`, and `Set`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/redundantheaderkey/cmd/redundantheaderkey
go tool redundantheaderkey ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -redundantheaderkey ./...
go tool lesiw.io/tools/cmd/fix -redundantheaderkey ./...
```
