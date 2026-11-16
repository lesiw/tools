# lesiw.io/tools/analyzers/queryadd

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/queryadd.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/queryadd)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an Add call on the values returned by
`URL.Query`.

```go
func add(u *url.URL) {
    u.Query().Add("some-key", "some-value") // the added value is discarded
}
```

The net/url documentation says that `Query` parses `RawQuery` and returns the
corresponding values, and that `Query` silently discards malformed value
pairs. `Query` builds a separate map, so a change to it does not affect
`RawQuery`. To change the URL, assign the encoded values to `RawQuery`:

```go
q := u.Query()
q.Add("some-key", "some-value")
u.RawQuery = q.Encode()
```

Encoding the map back into `RawQuery` drops the malformed pairs that `Query`
discarded.

The analyzer reports an Add call whose receiver is the `Query` result or a
conversion to `url.Values`, or whose first argument is the `Query` result. It
does not report an Add call on the result of a `Query` method other than
`(*url.URL).Query`, or on a variable. The variable may be encoded and assigned
back to `RawQuery`, so the call can take effect.

Report only: the two repairs are to delete the call and to declare a variable
for the values, call `Add` on it, and assign the result of `Encode()` to
`RawQuery`. Deleting the call can drop the evaluation of the receiver and
arguments, and the line may carry a comment. The encode fix makes the add take
effect, which changes behavior.

The rule is inspired by staticcheck's SA4027, which also reports `Set` and
`Del`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/queryadd/cmd/queryadd
go tool queryadd ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -queryadd ./...
```
