# lesiw.io/tools/analyzers/blankassign

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/blankassign.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/blankassign)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a statement that assigns the value of a
name or a basic literal to the blank identifier when the assignment has no
effect.

```go
func f(n int) {
    println(n)
    _ = n // assignment to the blank identifier has no effect
}
```

Effective Go says the blank identifier "can be assigned or declared with any
value of any type, with the value discarded harmlessly"
(https://go.dev/doc/effective_go#blank). Discarding the value is not an effect,
and a statement like `_ = x` does nothing but read `x`.

For a name, the rule reports a blank assignment when the code reads the name
again with no write in between. A use after a write reads a different value, so
it does not count, and deleting the blank assignment may let the compiler
eliminate the computation that produced the value. A use in another blank
assignment the fix can delete does not count either, since a fix that deleted
both could leave a local variable or an import unused. A name with no other use
is left alone as well: an ordinary local variable would fail the compiler's
unused check, and the assignment may be there for a linter.

A numeric or boolean variable is reported only when it is declared inside a
function, and a numeric or boolean named constant is reported wherever it is
declared. Both need another read; a predeclared constant needs none. A named
constant with no other read is left alone because the assignment may be there
for a linter. A variable of any other type can
hold a reference, whose lifetime a use can extend, so deleting the read could
let a finalizer run earlier; it can also be written through a selector, an
index, or a copied slice that the analysis does not track. A package-level
variable is left alone because its reads and writes can come from other
functions. A name used inside a function literal is left alone, since the
literal runs at its call. An assignment of a name whose address is taken is
left alone for the same reason. A basic literal or a predeclared constant is
reported without another use, since no variable's value is involved.

The right-hand side is an identifier, a basic literal, or a name qualified by a
package. The scan covers the package, so a read in another file counts for a
package-level constant.

Not reported:

- a call, a receive, an index expression, a dereference, or a field selection,
  since evaluating one can have an effect or panic;
- an assignment of a variable of any type other than a numeric or boolean,
  including a string, struct, array, pointer, slice, map, channel, function,
  interface, or type parameter;
- an assignment of a package-level variable;
- an assignment of a name used inside a function literal;
- an assignment of a name whose address is taken;
- a name with no other use, as with a local variable the compiler would report
  as unused or a name a linter would otherwise report as unused;
- a package-qualified name whose import the file uses nowhere else, since the
  import would become unused;
- an identifier from a dot import;
- a labeled statement, since deleting it would leave the label without a
  statement;
- an assignment with more than one target or value, as in `_, _ = f()`;
- a declaration of the blank identifier, as in `var _ = 0`.

Fixable: the statement is deleted, with its lines when it occupies them alone.
A comment inside the statement or on one of its lines, or a directive on the
line before it, is reported without a fix, since the edit would lose or detach
it.

The rule is inspired by staticcheck's S1005
(https://staticcheck.dev/docs/checks/#S1005), which drops unnecessary blank
identifiers in range clauses, multiple assignments, and channel receives. This
rule reports the `_ = name` and `_ = literal` forms that S1005 does not cover.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/blankassign/cmd/blankassign
go tool blankassign ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -blankassign ./...
go tool lesiw.io/tools/cmd/fix -blankassign ./...
```
