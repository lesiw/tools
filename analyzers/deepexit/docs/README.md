# lesiw.io/tools/analyzers/deepexit

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deepexit.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deepexit)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a program exit outside main or init.

```go
func serve(err error) {
    log.Fatal(err) // log.Fatal outside main or init ends the program
}
```

`os.Exit` ends the program at the call. The log package's Fatal functions
print, then call it. A function that calls one of them gives its caller no
result and no error.

`init` may end the program in any package, and `main` may in package `main`. A
method named `main` or `init` is reported, and so is a `main` in a package that
is not a program. So is a function that ends the program on a command's behalf,
such as a library's `Main`: the report does not care which frame of the program
holds the exit.

A test file is not reported: nothing ships it.

The rule reports a direct call of `os.Exit`, `syscall.Exit`, `log.Fatal`,
`log.Fatalf`, `log.Fatalln`, or the Fatal methods of a `*log.Logger`. The log
package's Panic functions are left alone, since a panic can be recovered. So is
`flag.Parse`, which ends the program only for a command line that asked for
help or did not parse.

A call the program reaches through a value is not reported: the callee is read
at the call site.

A call that invokes a literal where it is written is reported, as
`go func() { os.Exit(1) }()`; a call inside a literal that is kept or passed is
not. A package-level initializer is left alone, as `init` is.

Report only. There is no local edit: the function would have to hand the
failure to a caller, and only the caller knows what to do with it.

The idea comes from revive's deep-exit.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deepexit/cmd/deepexit
go tool deepexit ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deepexit ./...
```
