# lesiw.io/tools/analyzers/slowloris

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/slowloris.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/slowloris)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an `http.Server` literal with no timeout
for reading headers.

```go
func serve() error {
    srv := &http.Server{Addr: ":8080"} // no header read timeout
    return srv.ListenAndServe()
}
```

`ReadHeaderTimeout` is, in the `Server` documentation's words, "the amount of
time allowed to read request headers". A negative value, or a zero while
`ReadTimeout` is zero or negative, means no timeout, so a client that sends its
headers slowly holds the connection open. That is the slowloris attack.

`net/http` reads `ReadHeaderTimeout` unless it is zero, and `ReadTimeout`
otherwise, and sets a deadline only for a value above zero. The rule reads the
two the same way: a negative value is not a deadline, and a value the rule
cannot see counts as one.

A later assignment that gives a deadline suppresses the report:

```go
srv := &http.Server{Addr: ":8080"}
srv.ReadHeaderTimeout = time.Minute
```

The analyzer follows the path that holds the literal: the variable, the fields
it sits in, a dereference, or a keyed element of a struct literal. For a local
variable, only a write after the literal and before another server is assigned
to that path counts. A package-level server is written by whichever function
runs, so a value it is given anywhere in the package counts, and a later write
of zero does not clear it. A write inside a branch counts whether or not the
branch runs. A compound assignment or an increment counts, since the rule does
not compute what it leaves.

`a.server` and `b.server` are different paths, so `a = &app{server: ...}` takes
a later write to `a.server.ReadHeaderTimeout`.

The rule matches a path by the objects it names, so a write through a second
variable that holds the same server is not seen. Four shapes have this in
common:

- a server passed to a function that configures it;
- one a function returns, which its caller then configures;
- one held in a slice or map element, which no path names;
- one reached through a second variable that holds the same pointer.

A literal in one of those shapes is reported where it is written, so a builder
that leaves the timeout to its caller is reported at the builder.

Not reported, and why:

- a struct with a field named `ReadHeaderTimeout` that is not
  `net/http.Server`;
- `new(http.Server)` and `var srv http.Server`: a zero-value server has the
  same fault, and the rule reads literals;
- a literal in a test file, which the rule tells by its name. The only client a
  test serves is the test, so test support in a plain file is reported.

`http.Serve`, `http.ServeTLS`, `http.ListenAndServe`, `http.ListenAndServeTLS`
and `net/http/httptest`'s constructors build a server of their own, so a call
site holds no literal and is not reported.

Report only: any duration would be a guess at the server's policy, so there is
no fix.

The rule is adapted from gosec's G112. It differs from that check in reading the
two fields in `net/http`'s order, and in following the whole path rather than
the last field.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/slowloris/cmd/slowloris
go tool slowloris ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -slowloris ./...
```
