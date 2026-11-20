# lesiw.io/tools/analyzers/unusedfunc

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unusedfunc.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unusedfunc)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a package-level declaration nothing in the
package refers to.

```go
func helper() {} // function "helper" is unused

type row struct{} // type "row" is unused

const retries = 3 // const "retries" is unused
```

A declaration counts as unused when it is unexported and every reference to it
sits inside the declaration itself, as a recursive call does. A reference in
another file of the package is a use, and so is a method selection.

The rule stands down where the package's own source cannot see a use. An
exported name may be referred to from another package, and a blank identifier
declares nothing at all. A method whose name matches a method an interface in
the package declares may be called through that interface; an unexported name
cannot be satisfied from another package, so an interface declared elsewhere
constrains nothing. `init` and `main` are called by the runtime. A declaration
whose doc comment carries a `//go:linkname` directive is named from another
package.

Six callers are invisible to any rule that reads one package: assembly calls a
function by name, another package may reach one through a `//go:linkname`
directive, a file the build excludes may hold the only call, a program may look
a name up at run time, C calls one through cgo's `//export`, and a WASM host
calls one through `//go:wasmexport`. The rule sees only the directives written
on the declaration. A declaration that carries `//go:linkname` is left alone,
since the directive says another package owns the name. The rest are reported,
and the paragraph on fixes says which of them carry an edit. A package that
builds some of its files for another platform therefore reports the
declarations only those files call, and offers to delete them. The Go toolchain
is the one tree left alone, because those callers are common there.

A set of constants that all share one type is reported once, as a set, and only
when none of its names is used, since some of the values are often there to
keep the group symmetric or its ordering intact.

Fixable: the declaration is deleted, with its doc comment and any comment on the
line it ends on. A block whose names are all unused goes whole rather than being
emptied. A var or const specification that declares another name, or whose
values have an effect, keeps what it needs. The reported name goes with the
value paired with it, or alone with its comma when the specification has no
values. It becomes `_` when something else has to stay: a value that has an
effect, a constant below that counts specifications with `iota`, or a
specification below that repeats these values.

A specification takes one edit per run: the first name reported in it carries
the edit, and the names beside it follow when the tool runs again.

Four diagnostics carry no fix. A name that a value in its own specification
reads, as `var total, doubled = 3, total * 2` does, cannot go while the
specification stays. A function whose doc comment carries `//export` or
`//go:wasmexport` is named by C or by a WASM host. A function with no body is
the declaration that assembly implements.

This is vendored from gopls' `unusedfunc`
(`gopls/internal/analysis/unusedfunc`, `golang.org/x/tools/gopls` v0.24.0),
which is internal and cannot be imported. The rule is upstream's; the code is
adapted to this repository's helpers. The fixes are written here: upstream
would delete a specification that the one below it repeats, leaving a constant
declaration with nothing to repeat.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unusedfunc/cmd/unusedfunc
go tool unusedfunc ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unusedfunc ./...
go tool lesiw.io/tools/cmd/fix -unusedfunc ./...
```
