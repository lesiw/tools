# lesiw.io/tools/analyzers/ctxname

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ctxname.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ctxname)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports `context.Context` variables
that can be named `ctx`.

One value threads through nearly every call chain in modern Go,
and one name keeps its path scannable: a context under any other
name is a context a reader has to track by hand.

## Checks

### Contexts are named ctx

A function-scoped variable, parameter, or named result of type
`context.Context` should be named `ctx`:

```go
func f(c context.Context) { // context.Context should be named ctx, not c
    use(c)
}
```

The diagnostic applies only when a rename provably cannot rebind
anything: no identifier spelled `ctx` — including a selector's
field name — appears in the enclosing function after the
candidate's declaration or lives before it without a legal
redeclaration, and no sibling candidate shares the function. A
second context alongside a live `ctx` is the one legitimate case
for another name, so it is not reported at all. References inside
the candidate's own declaration statement stay on the outer
binding — `ctx, cancel := context.WithTimeout(ctx, d)` is the
canonical shadow — so they never block the rename.

A struct field of type `context.Context` should also be named
`ctx`, as the standard library names its own (`http.Request`,
`testing.T`); a field is reported unless a sibling field already
claims the name.

The `parent` parameter of a context-deriving function — the
standard library's own `WithCancel(parent Context)` shape — keeps
its name. Package-scope variables and parameter names in interface
methods and function-type declarations are not reported — an
interface method need not name its parameters at all — and a
variable whose declared type is an alias of `context.Context` is
not matched.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ctxname/cmd/ctxname
go tool ctxname ./...
```
