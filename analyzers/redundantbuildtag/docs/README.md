# lesiw.io/tools/analyzers/redundantbuildtag

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/redundantbuildtag.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/redundantbuildtag)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `//go:build` line the file name already
implies.

```go
//go:build linux

package a
```

In `a_linux.go`, the line is reported, and the fix removes it.

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

A file whose only constraint is a `// +build` line is left alone: gofmt writes
the `//go:build` line from it, and the next run has something to report.

The rule reports a file only in a build that includes it, so a name another
platform compiles is left to that platform's run.

The rule is adapted from revive's redundant-build-tag.

Fixable: the line is removed, and the blank line after it when the line is the
whole of its comment group. A `// +build` line beside it goes as well: the
build ignores those while a `//go:build` line is present, and gofmt would write
the `//go:build` line back from one, so the redundancy would return.

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
