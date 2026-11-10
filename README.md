# lesiw.io/tools

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools.svg)](https://pkg.go.dev/lesiw.io/tools)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](LICENSE)

Commands for working on Go projects.

```sh
go run lesiw.io/tools/cmd/fix@latest ./...
```

## cmd/vet

Vet checks Go packages with the golang.org/x/tools analysis passes. It adds
analyzers for error handling, line length, layout, naming, testing practice,
declaration style, and code modernization. A //ignore directive suppresses a
diagnostic, for one analyzer or for all of them. Each analyzer can be selected
or disabled by name:

```sh
go tool lesiw.io/tools/cmd/vet -printf=false ./...
go tool lesiw.io/tools/cmd/vet -printf=true ./...
```

An analyzer's own flags are namespaced under its name, such as
-linelen.len=100.

Run it by its full package path. The short name refers to the Go
distribution's own vet tool:

```sh
go tool lesiw.io/tools/cmd/vet ./...
```

The -fix flag applies the suite's suggested fixes:

```sh
go tool lesiw.io/tools/cmd/vet -fix ./...
```

## cmd/fix

Fix applies the analyzer suite's fixes over the packages named on the command
line, and repeats until nothing is left to suggest.

The analyzers run in this process. A round loads the packages, runs the suite
over them, and writes the edits every analyzer suggested, then formats the
result and prunes the imports the fixes orphaned. A round that finds nothing is
a fixed point. A fix is applied whole or not at all: when two analyzers would
edit the same span, one of them waits for the next round.

A package that does not load stops the run, because the next round cannot
check a fix applied to a tree that does not compile.

Analyzer flags mean what they mean to the analyzer: `-errfmt` runs one
analyzer, `-errfmt=false` turns one off, and `-linelen.len=100` sets one of
its own. Build flags are passed to the load.

## cmd/reflow

Reflow fills comment prose to the width limit. It also lifts a trailing comment
that is too long for its line above the code it trails. Lifting moves a comment
and does not shorten it. It reads source on stdin, and it changes
only the comments. The file passes through the printer, so a file gofmt would
change comes back gofmt-clean:

```sh
go tool lesiw.io/tools/cmd/reflow < in.go > out.go
```

It is a separate command because it moves the line breaks inside comments.
Reflow joins a paragraph's lines and breaks them again at the width limit. An
author who breaks those lines on purpose loses them.

## cmd/clerk

Clerk writes standard project scaffolding into the current
directory:

```sh
go tool lesiw.io/tools/cmd/clerk app
```

The project types are:

* **app** — a Go application. Writes a check workflow that checks
  formatting, vets, and tests the module through
  [lesiw.io/gorc](https://pkg.go.dev/lesiw.io/gorc/go) and
  cross-compiles it, plus an `.editorconfig` with Go formatting
  settings.
* **lib** — a Go library. The same, tested against the two most
  recent Go releases.

Clerk records a checksum for every file it writes to a clerk.sum
file, managed with [lesiw.io/clerk](https://pkg.go.dev/lesiw.io/clerk).
Files that still match their recorded checksum are updated in
place on later runs; clerk prompts before overwriting or deleting
any file that changed since it was last written.

## go.rc

With [lesiw.io/gorc](https://pkg.go.dev/lesiw.io/gorc/go)
installed, a project can bind these commands to go verbs in its
go.rc:

```
vet go tool lesiw.io/tools/cmd/vet $GOARGS $GOFLAGS
fix go tool lesiw.io/tools/cmd/fix $GOARGS $GOFLAGS
clerk go tool lesiw.io/tools/cmd/clerk app
```
