# lesiw.io/tools/analyzers/builtinprint

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/builtinprint.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/builtinprint)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports calls to the predeclared print and println
functions.

```go
func f(n int) {
	println("step", n) // println can be replaced by fmt.Fprintln(os.Stderr, ...)
}
```

Go's `builtin` package documents both as bootstrapping aids, formats their
arguments in an implementation-specific way, and says they are "not guaranteed
to stay in the language". They write to the process's standard error. The
rewrite writes to `os.Stderr`, which a program may have replaced.

```go
fmt.Fprintln(os.Stderr, "step", n)
```

The fix adds the import the file is missing as a declaration after the package
clause. `cmd/fix` folds that into the file's own block, and a driver that only
formats the file still writes code that compiles.

The rule reports only in packages that belong to a module. The standard
library, the compiler, and a tree built without modules report no module, and
their calls to the builtin are deliberate.

## When the fix is withheld

A call is reported and left without a fix where fmt would write something else:

- An operand is not a boolean, a number, or a string. The builtin prints the
  value itself. fmt calls a method named Error, Format, or String on it first,
  prints the value a pointer points to, and prints a slice's elements.
- print is given two adjacent operands that are not strings. Fprint separates
  those with a space, and print runs them together.

The comparison is against gc's print routines, in `runtime/print.go`, since
neither the language nor the runtime promises the builtin's output. A
`time.Duration` is the shape worth knowing about: it is an int64 to the builtin
and a string to fmt, so the author chooses between `fmt.Fprintln(os.Stderr,
int64(d))` and fmt's rendering.

A call is reported with its fix withheld where the file cannot take the
rewrite:

- A declaration or another import holds a name the rewrite writes.
- The file binds fmt or os to the dot or blank name.

The diagnostic stands wherever the fix does not: the call still depends on
something Go does not promise.

The idea comes from revive's use-fmt-print.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/builtinprint/cmd/builtinprint
go tool builtinprint ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -builtinprint ./...
go tool lesiw.io/tools/cmd/fix -builtinprint ./...
```
