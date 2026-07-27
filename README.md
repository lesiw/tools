# lesiw.io/tools

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools.svg)](https://pkg.go.dev/lesiw.io/tools)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](LICENSE)

Commands for working on Go projects. Each command lives under
cmd and is added to a project as a module tool:

```sh
go get -tool lesiw.io/tools/cmd/vet
go get -tool lesiw.io/tools/cmd/clerk
```

## cmd/vet

Vet checks Go packages with the golang.org/x/tools analysis
passes plus analyzers for error handling, line length, deprecated
APIs, and code modernization. Diagnostics may be suppressed with
//ignore directives as supported by
[lesiw.io/checker](https://pkg.go.dev/lesiw.io/checker).

Run it by its full package path — the short name would refer to
the Go distribution's own vet tool:

```sh
go tool lesiw.io/tools/cmd/vet ./...
```

Results are cached by the go build cache.

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
clerk go tool lesiw.io/tools/cmd/clerk app
```
