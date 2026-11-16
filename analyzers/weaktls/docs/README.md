# lesiw.io/tools/analyzers/weaktls

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/weaktls.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/weaktls)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `tls.Config` literal that disables
certificate verification or permits TLS below 1.2.

```go
func skip() *tls.Config {
    return &tls.Config{InsecureSkipVerify: true} // InsecureSkipVerify accepts any certificate and host name
}
```

The `crypto/tls` documentation for `Config.InsecureSkipVerify` says that when
it is true, `crypto/tls` accepts any certificate presented by the server and
any host name in that certificate, and that TLS is then susceptible to
machine-in-the-middle attacks unless custom verification is used. The
documentation for `Config.MinVersion` says the zero value uses TLS 1.2 as the
minimum. `InsecureSkipVerify` is a client setting; the rule reports it in any
`tls.Config` literal, because the literal does not show whether it configures
a client or a server.

The rule reports the two fields when their values are constants:
`InsecureSkipVerify` set to true, and a nonzero `MinVersion` below
`tls.VersionTLS12`. A zero `MinVersion` selects the TLS 1.2 default.

An `InsecureSkipVerify` report is suppressed when the same literal sets
`VerifyConnection` to an expression other than nil. A literal that sets
`VerifyPeerCertificate` suppresses the report only when it also sets
`SessionTicketsDisabled` to the constant true, because that callback is not
invoked on a resumed connection. The rule does not read the callback, and a
`VerifyConnection` callback held in a variable counts. A nil literal does not
count, including a conversion of nil to the callback's type. On a client with
a nil `ClientSessionCache`, resumption is also disabled, but whether the config
is a client or a server is not visible from the literal, so the rule requires
`SessionTicketsDisabled`. The suppression covers only `InsecureSkipVerify`; a
low `MinVersion` is still reported.

A `_test.go` file is left alone: the rule skips the whole file, because a test
config is not part of the built program and the documentation says
`InsecureSkipVerify` is for testing.

The rule reads the keyed fields of a composite literal. An assignment to a
field after the literal, as in `cfg.VerifyConnection = verify`, is not read:
that callback could make the config secure.

A non-constant `InsecureSkipVerify` or `MinVersion` is left alone, because its
value is not visible to the tool. A type that has fields with the same names is
left alone; the rule checks the type `crypto/tls.Config`. `MaxVersion` is not
checked, because the rule reports a minimum version.

Report only: removing the field or raising the version makes a connection to an
unverified server or to an older protocol fail, and the author's intent is not
visible to the tool.

The rule is adapted from gosec's `G402`. gosec checks cipher suites,
`MaxVersion`, and `PreferServerCipherSuites` too, reports an
`InsecureSkipVerify` value it cannot resolve, and reads an assignment as well
as a composite literal. `weaktls` reads the constant value with `go/types`,
reports only the two fields, and reads literals only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/weaktls/cmd/weaktls
go tool weaktls ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -weaktls ./...
```
