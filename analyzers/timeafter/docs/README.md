# lesiw.io/tools/analyzers/timeafter

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/timeafter.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/timeafter)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports timer ceremony a channel receive
can replace.

Since Go 1.23 the collector reclaims unreferenced timers before they
fire, so the historical leak that justified `NewTimer` plus a
deferred `Stop` around a single receive is gone.

## Checks

### Timer ceremony can be replaced by time.After

A `time.NewTimer` whose only uses are at most a deferred `Stop` and
one `<-timer.C` receive in a `select` beside the declaration:

```go
timer := time.NewTimer(time.Second)
defer timer.Stop()
select {
case v := <-ch:
    use(v)
case <-timer.C: // timer ceremony can be replaced by time.After
    return errTimeout
}
```

A timer that is `Reset`, stopped conditionally, or received anywhere
else is a real timer and is left alone. A `select` nested deeper
than the declaration's own statement list — inside a loop, most
commonly — is also left alone: `time.After` there would restart the
deadline every iteration. Only the `timer := time.NewTimer(d)` form
is examined. Statements may run between the declaration and the
`select`; moving the deadline into the receive starts it later,
which is the author's call.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/timeafter/cmd/timeafter
go tool timeafter ./...
```
