# lesiw.io/tools/analyzers/ptrtoerror

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ptrtoerror.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ptrtoerror)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a concrete error type that a package
converts to error both as a value and as a pointer.

A type assertion tests for the form it names. A package that writes both leaves
the caller no form that always matches.

## Checks

### A type converted both ways

```go
type mixed struct{ error }

func first(m mixed) error   { return m }  // mixed is converted to error both as a value and as a pointer
func second(m *mixed) error { return m }  // mixed is converted to error both as a value and as a pointer
```

Every conversion is reported, and each report names one conversion in the other
form, so the author picks a form and rewrites the rest. Neither report carries a
fix: rewriting a conversion changes which callers match, and the form is the
author's choice.

The rule reads a conversion wherever it can happen: an assignment, a
declaration, an argument, a variadic argument, a result, an explicit
conversion, an element, a key or a field of a literal, a channel send, a type
assertion, a type switch case, and the type argument of an instantiation such
as `errors.AsType[mixed]`.

### A type used as neither form

```go
type undecided struct{ error } // both undecided and *undecided implement the error interface, making the intent ambiguous
```

Both forms implement error, and the package converts neither one, so nothing
says which form was meant. The fix declares the value form:

```go
var _ error = *new(undecided)
```

`*new(undecided)` needs only the type's name, so the declaration works for a
type of any shape, and the type itself implements error.

A package that meant the pointer form writes that declaration instead:

```go
var _ error = (*undecided)(nil)
```

A generic type is reported without a fix, because its type parameters are not in
scope where the declaration would go.

### A conversion that disagrees with the declaring package

When a package converts its own exported type to error, that conversion decides
the form for the packages that import it. A conversion of the other form in
another package is reported with the file, line and column where the form was
decided:

```go
var p *client.Boom
var _ error = p // conversion of *client.Boom to error, but package "client" uses Boom (sans pointer) as an error (e.g. at client.go:12:16)
```

A package that never converts its own type to error records no form for it, so a
conversion of it elsewhere is not reported.

## Exemptions

- An alias gets no report of its own: the type it names is the one declared, and
  a conversion written through the alias is reported against that type.
- An interface type is left alone: the report would say that a pointer to it
  implements error too, and a pointer to an interface has no methods.
- An unnamed type is left alone: a conversion of `&struct{ error }{}` has no
  declaration to report at.

## Provenance

Vendored from gopls' `ptrtoerror` (`gopls/internal/analysis/ptrtoerror`,
`golang.org/x/tools/gopls` v0.24.0), which is internal and cannot be imported.
The rule is upstream's, and the code is adapted to this repository's helpers.
Upstream offers two fixes for the ambiguous type, one per form; this offers the
value form alone, because `cmd/fix` applies every fix a diagnostic suggests, not
one of them.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ptrtoerror/cmd/ptrtoerror
go tool ptrtoerror ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -ptrtoerror ./...
go tool lesiw.io/tools/cmd/fix -ptrtoerror ./...
```
