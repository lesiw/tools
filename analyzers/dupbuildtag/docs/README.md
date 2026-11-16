# lesiw.io/tools/analyzers/dupbuildtag

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dupbuildtag.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dupbuildtag)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `// +build` line whose condition an
earlier line in the same file already gives.

```go
// +build linux
// +build linux

package a
```

The build-constraint section of the go command's documentation says "It is an
error for a file to have more than one //go:build line"
(https://pkg.go.dev/cmd/go#hdr-Build_constraints). The design document for the
newer syntax says multiple legacy `// +build` lines "were in effect AND'ed
together"
(https://go.googlesource.com/proposal/+/master/design/draft-gobuild.md). A line
that repeats an earlier one therefore changes nothing and is usually a
copy-paste error.

Two conditions are the same when they differ only in the order of terms joined
by `&&` or `||`. So `// +build linux amd64` and `// +build amd64 linux` are the
same, and so are `// +build linux,amd64` and `// +build amd64,linux`. A line
whose condition differs from every earlier line is left alone, so
`// +build linux amd64` and `// +build linux,amd64` are distinct.

Only `// +build` lines are read. The go command rejects a second `//go:build`
line in a file it builds, and the suite's `buildtag` pass reports one in a file
the build ignores, so this rule does not read `//go:build` lines. A `// +build`
line is read whether or not the file also holds a `//go:build` line: a
repeated line is still a copy-paste error, and `plusbuild` reports only the
first obsolete line in a group. A line with an invalid tag is left alone.

The go command reads a legacy line only in the header: the run of blank lines
and `//` comments before the first non-`//`, non-blank line. A `// +build` line
in the package doc comment, or next to the package clause without a blank line
after it, is an ordinary comment and is not read. A `/* */` comment ends the
run. A generated file is left alone, since an edit would change generated text.

The rule reads the files the current build includes. A file the build excludes
is not offered to the analyzer, so a duplicate there waits for a run on a
platform that includes it. A non-Go file is not read, even though the go
command reads its constraints.

The diagnostic is reported at the package clause.

Fixable: the fix removes the repeated line, which cannot change the condition:
the duplicate is either ANDed with the earlier line or ignored. No fix is
offered when the source on disk does not hold the comment at the offset where
it was parsed, since the edit would remove different bytes.

The rule is adapted from staticcheck's `SA4019`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dupbuildtag/cmd/dupbuildtag
go tool dupbuildtag ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dupbuildtag ./...
go tool lesiw.io/tools/cmd/fix -dupbuildtag ./...
```
