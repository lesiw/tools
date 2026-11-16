# lesiw.io/tools/analyzers/redundantbuildtag

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/redundantbuildtag.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/redundantbuildtag)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `//go:build` line the file name or the
module's Go version already implies.

```go
//go:build linux

package a
```

In `a_linux.go`, the line is reported, and the fix removes it.

```go
//go:build go1.21

package a
```

In a module that declares `go 1.21`, the line clamps to the module's version
and is reported.

Go's build constraint documentation, `go help buildconstraint`, makes a file
name that matches `*_GOOS`, `*_GOARCH` or `*_GOOS_GOARCH` part of the
constraint: "the file is considered to have an implicit build constraint
requiring those terms (in addition to any explicit constraints in the file)".
The name and the line are combined with AND, and the build sets the tag the
name gives wherever it uses the file, so a condition that holds on that tag
alone states nothing. A compound condition goes too when the name settles it:
`//go:build linux && amd64` is redundant in `a_linux_amd64.go`.

A tag the name leaves open stands, so `//go:build linux && cgo` stays in
`a_linux.go`. The name's suffix is matched as a tag, so `-tags=linux` builds
`a_linux.go` on Windows. The rule therefore says nothing about a tag the name
does not spell: `//go:build unix` in `a_linux.go` and `//go:build !unix` in
`a_windows.go` are left alone. A GOOS that implies another is not enough
either, since `-tags=android` matches an `android` name on any system, where
`linux` is unset.

The go command sets the release tags `go1.1` through `go1.N` for the toolchain
version 1.N, and a `//go:build` line can name one of them. Since Go 1.21 the
`go` directive is a minimum version, so a module at Go 1.21 or later is built
only by a toolchain that sets every release tag up to the module's version. A
`go1.X` tag at or below that version holds in every build.

Go also uses the `//go:build` line to set the file's language version, clamped
to Go 1.21. The rule removes a line whose version settles the condition only
when the line sets no version, so removing it changes nothing, or when its
version after the clamp is the module's, so the file is already at the module's
version. A line whose version after the clamp is not the module's is left
alone, since dropping it would move the file to the module's version. A module
below Go 1.21 is left alone too, because its `go` directive was advisory. A
tag that is not one of the `go1.N` release tags is left alone as well, since
the go command sets no such tag.

Leaves alone: a file the current build does not include, since the go command
does not offer it and a run on the platform the name selects reports it. A
`// +build` line in a file that is not reported for another reason is left
alone too, since the suite's `plusbuild` pass reports that line in a file at Go
1.18 or later and both analyzers would then report the same line. A `// +build`
line in a group with another comment is left alone, since it may be
documentation; gofmt writes the `//go:build` line back from it, and the next run
has the same line to report. The rule also leaves a file whose only constraint
is a `// +build` line, since gofmt writes the `//go:build` line from it and the
next run has something to report.

The rule is adapted from revive's redundant-build-tag.

Fixable: the line is removed, and the blank line after it when the line is the
whole of its comment group. A `// +build` line in a group that holds only build
constraints goes as well: the build ignores those while a `//go:build` line is
present, and gofmt would write the `//go:build` line back from one, so the
redundancy would return.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/redundantbuildtag/cmd/redundantbuildtag
go tool redundantbuildtag ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -redundantbuildtag ./...
go tool lesiw.io/tools/cmd/fix -redundantbuildtag ./...
```
