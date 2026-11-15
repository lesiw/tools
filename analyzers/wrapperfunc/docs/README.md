# lesiw.io/tools/analyzers/wrapperfunc

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/wrapperfunc.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/wrapperfunc)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a declared function that only forwards its
arguments.

```go
func outer(x int) int { // func outer only forwards its arguments; call inner instead
    return inner(x)
}
```

The body is one call that forwards the parameters, in order, to another
function of the same signature.

Report only: the callers would each have to be rewritten, and a caller's file
might not already import the callee. A wrapper kept for that import boundary is
a layering decision the report does not make, and its doc comment is where the
reason goes.

The rule stands down where deleting the wrapper would lose something the body
cannot show:

- an exported function or method, which another package may call or use to
  satisfy an interface;
- a method whose name and signature an interface in the package declares;
- a function the package holds as a value, where the name is read at a site
  that is not a call;
- a function with a doc comment, or with a comment inside its body, since
  deleting the wrapper takes the comment with it. A directive alone is not a
  doc comment, except one that publishes the name outside the package:
  `//go:linkname`, `//go:linknamestd`, `//go:wasmexport`, and `//export`;
- init and a program's main, which nothing can call;
- a method value, which carries its receiver, so the name at a call site is the
  caller's receiver rather than one the package declares. A method expression
  is reported, since its receiver is one of the wrapper's own parameters.

A callee that is a builtin, a conversion, a struct field, or a func-typed
variable is not a function declaration, so there is no declared callee to name.

The rule reads only the files this configuration loads. A function in a test
file is left alone, since a test's helper is not shipped. So is every
declaration in a file that a build constraint or a platform suffix could
replace with another configuration's file, since the declarations and holds
that configuration would add are not visible here.

The rule cannot see a name the toolchain looks up itself: the compiler calls
`runtime.fmin32` for a float minimum and `runtime.fint64to32` for a conversion,
and a `//go:linkname` in another package can bind a target here. Keep the
wrapper and add an `//ignore` on a function the toolchain names this way; the
compiler names the wrapper, so no call site can be edited.

The name comes from go-critic's wrapperFunc, which reports calls an existing
convenience wrapper replaces.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/wrapperfunc/cmd/wrapperfunc
go tool wrapperfunc ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -wrapperfunc ./...
```
