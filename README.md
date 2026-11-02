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
analyzers for error handling, line length, testing practice, declaration
style, and code modernization. A //ignore directive suppresses a diagnostic,
for one analyzer or for all of them. Each analyzer can be selected or
disabled by name:

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

The go command does the work. Each round asks `go vet` which analyzers still
have fixes, then runs one analyzer at a time with `-fix`, so two analyzers
never edit the same file in one pass. That is the shape whose work is never
discarded: the go command keeps the patch of a tool that exits zero, and a tool
that sets a fix aside exits non-zero.

Every flag goes to the go command, so an analyzer flag means there what it
means there: `-stringsseq` runs one analyzer, `-stringsseq=false` turns one
off, and `-linelen.len=100` sets one of its own.

## cmd/fmt

Fmt lays Go source out under stricter rules than gofmt. It puts a wrapped call
or control-flow header back onto one line when the result fits. It rejoins a
composite literal whose elements share a line. A parenthesized block holding
one spec loses its parentheses. A declared signature stays on one line however
long it is. Fmt sets the blank lines between declarations and before a doc
comment inside a block. It takes gofmt's `-l` and `-w`:

```sh
go tool lesiw.io/tools/cmd/fmt -w .
```

A file marked generated passes through untouched. One pass makes the output
gofmt-clean. Fmt never breaks a long line, and never makes one longer than
[lesiw.io/linelen](https://pkg.go.dev/lesiw.io/linelen) allows. It does move a
closer onto a line of its own when its opener sits at a different indent.

## cmd/reflow

Reflow fills comment prose to the width limit. It also lifts a trailing comment
that is too long for its line above the code it trails. Lifting moves a comment
and does not shorten it. It reads source on stdin, like cmd/fmt, and it changes
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
