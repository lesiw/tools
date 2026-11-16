# lesiw.io/tools/analyzers/deleteguard

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deleteguard.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deleteguard)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an existence check around a delete of the
same key.

```go
func forget(rows map[string]int, name string) {
    if _, found := rows[name]; found { // delete does not need a presence check
        delete(rows, name)
    }
}
```

The documentation for `delete` says "If m is nil or there is no such element,
delete is a no-op." The check therefore does not affect the delete.

The init reads the map value into a blank identifier and the key's presence
into a flag, the condition tests that flag, and the body contains one call to
`delete` as its only statement. The call deletes the same key the index read.
An inverted check, as in `if _, found := rows[name]; !found`, is left alone,
since dropping the guard would delete a key that is present. A condition that
is not exactly the flag is left alone as well. An init that assigns to
existing names with `=`, a body with another statement, and an else are also
left alone.

The map and the key are compared by the object each name resolves to, so a
shadowed name is a different value. Identifiers, fields, indexes, basic
literals, and dereferences are compared one by one. A call is left out, because
two calls may differ, so a check on `rows[next(name)]` and a delete of
`rows[next(name)]` are left alone. A binary expression, a unary expression
other than a dereference, a composite literal, and a type assertion are left
alone too, as is a `delete` that resolves to a declaration other than the
builtin.

The rule is adapted from staticcheck's S1033, which reports the same guard.

Fixable: the if becomes the delete in its body. A delete written on several
lines is written on one line; where that line would run past the length limit,
the fix is withheld, unless the guard's first line is already past the limit.
With no width set, every line fits.

```go
func forget(rows map[string]int, name string) {
    delete(rows, name)
}
```

An else-if with no else becomes an else block that contains the delete.
`} else if _, found := rows[name]; found { delete(rows, name) }` becomes:

```go
} else {
    delete(rows, name)
}
```

A comment on its own line above the if stays above the delete.

The analyzer reports these cases without a fix:

- a comment inside the if, on the if's first or closing line, or between the
  branch before it and the if, where the edit would delete it or leave it
  describing the check;
- a delete with no one-line form, as a raw string that contains a newline has
  none;
- a line the fix writes that would run past the width `linelen` is set to,
  unless the guard's first line is already past the limit; with no width set,
  every line fits.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deleteguard/cmd/deleteguard
go tool deleteguard ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deleteguard ./...
go tool lesiw.io/tools/cmd/fix -deleteguard ./...
```
