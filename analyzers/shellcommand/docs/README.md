# lesiw.io/tools/analyzers/shellcommand

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/shellcommand.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/shellcommand)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to `exec.Command` or
`exec.CommandContext` whose program name is a constant that contains
whitespace.

```go
exec.Command("alpha --one") // program name contains whitespace; the whole string is one file name
```

The `os/exec` package "intentionally does not invoke the system shell." The
program name is one file name, and every argument after it is handed to the
program unchanged, so the call above looks up `alpha --one` as one name and
fails when it runs because no executable has that name.

```go
exec.Command("alpha", "--one")
```

A command line that needs a shell is passed as one argument to that shell:
`exec.Command("sh", "-c", line)`, though not every system has `sh`.

A path may contain a space: `exec.Command("/opt/my tools/alpha")` names one
file and runs. The rule reports it anyway, because a separator does not tell
the shapes apart: `exec.Command("/bin/ls -l")` is a command line with its space
in the same place, and `exec.Command("ls -l /tmp")` one with the space in the
other. Only a constant is examined, so a name held in a variable is left alone.

The rule is adapted from staticcheck's SA1005, which examines `exec.Command`,
leaves a name that holds a path separator alone, and looks only for an ASCII
space. This analyzer examines `exec.CommandContext` too, reports a name that
holds a path separator, and tests for whitespace.

Report only: the call does not say whether the author meant a shell line or one
file name.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/shellcommand/cmd/shellcommand
go tool shellcommand ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -shellcommand ./...
```
