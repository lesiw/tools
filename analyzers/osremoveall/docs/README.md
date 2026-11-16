# lesiw.io/tools/analyzers/osremoveall

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/osremoveall.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/osremoveall)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports `os.Remove` or `os.RemoveAll` on a path
that a variable leaves at the filesystem root.

```go
func removeDir(dir string) {
    os.RemoveAll(dir + "/") // os.RemoveAll path built from a variable may be the filesystem root
}
```

[`os.RemoveAll`](https://pkg.go.dev/os#RemoveAll) removes a path and any
children it contains, and [`os.Remove`](https://pkg.go.dev/os#Remove) removes a
file or an empty directory. `dir + "/"` names `dir/` when `dir` has a value and
`/` when `dir` is empty, so the call reaches the filesystem root instead of a
directory under the caller's tree.

The rule reads a string concatenation or a `path/filepath.Join` or `path.Join`
call. It reports when the path is the filesystem root once the variables are
empty. `dir + "/"`, `dir + "/.."`, and `filepath.Join(dir, "/")` are reported.
A literal directory name keeps the path below the root, so `dir + "/cache"` is
left alone. A leading separator is reported too: `"/" + dir` and
`filepath.Join("/", dir)` name the root when `dir` is empty. A plain variable
and a path returned by any other call, such as `fmt.Sprintf`, are left alone.

A guard on the whole path proves it is not the root. A guard on the only
variable also settles a path with no leading separator and no dot component.
A string constant assigned to the last variable settles the path when the
constant keeps the path below the root, as `name := "cache"` does. The
conditions the rule reads are a comparison with `""`, a comparison of `len`
with a constant, and a comparison of `filepath.Clean` or `path.Clean` with the
root. A direct comparison with `"/"` is left out: `"//"` and `"/tmp/.."` also
name the root, so the rule compares the cleaned value. The rule reads the
condition of an enclosing if or else branch, a tagless switch case, and a
preceding if statement whose branch ends with `return`, `panic`, `break`, or
`continue`. A later assignment to the variable ends the guard. So does a
function literal: a guard outside it does not apply inside it, since the
literal can run after the variable changes.

The rule does not follow a value that arrives from a call or from another
variable. A variable is replaced with the empty string in the path model; a path
that is the root only when a variable holds the root is left alone.

The path is reported on the argument, and the message names the call:

```
os.RemoveAll path built from a variable may be the filesystem root
```

Report only: the repair is a guard that the variable is neither empty nor the
root, or a path that stays relative. The rule is adapted from staticcheck's
SA9007.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/osremoveall/cmd/osremoveall
go tool osremoveall ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -osremoveall ./...
```
