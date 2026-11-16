# lesiw.io/tools/analyzers/hostportconst

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/hostportconst.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/hostportconst)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a constant address that a listen function
cannot parse as `host:port`.

```go
func listen() (net.Listener, error) {
    return net.Listen("tcp", "alpha-one")
}
```

For the tcp and udp networks, the address has the form `host:port`. The host,
the port, or both can be empty, and a literal IPv6 host is enclosed in
brackets.
`net.SplitHostPort` parses those forms, and the port is a number from 0 to
65535 or a service name. An address that fails either check cannot work, and a
constant is the same on every run.

The functions checked are `net.Listen`, `net.ListenPacket`,
`(*net.ListenConfig).Listen`, `(*net.ListenConfig).ListenPacket`,
`http.ListenAndServe` and `http.ListenAndServeTLS`. Only a direct call is
checked; a call through a function value is left alone. The network is an
argument to the `net` functions, and only a constant `tcp`, `tcp4`, `tcp6`,
`udp`, `udp4` or `udp6` network is checked: for a unix or IP network, the
address has another form, so the call is left alone. A call whose network is
not a constant string is left alone too. The host is not resolved, and a
service name is not looked up: an address that parses but names a host or
service that does not exist is not reported.

Report only: the intended address is not known to the tool.

The rule is adapted from staticcheck's `SA1020`, which checks only the net/http
functions.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/hostportconst/cmd/hostportconst
go tool hostportconst ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -hostportconst ./...
```
