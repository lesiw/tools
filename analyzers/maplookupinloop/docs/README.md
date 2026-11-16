# lesiw.io/tools/analyzers/maplookupinloop

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/maplookupinloop.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/maplookupinloop)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a map lookup in a loop whose map and key do
not change.

```go
func f(m map[string]int, keys []string) {
    for range keys {
        _ = m["alpha"] // map lookup does not depend on the loop; evaluate it once before the loop
    }
}
```

The spec's section on [index
expressions](https://go.dev/ref/spec#Index_expressions) says a map index yields
the element for a key. A lookup that reads the same map with the same key
returns the same element on every pass, so every pass in the body can use one
value read before the loop.

The loop is a for or a range statement, and the lookup is in its body. The map
expression is a name or a field path that does not go through a pointer. The
key is a name, a literal, a field path that does not go through a pointer, or
an expression built from those with operators, a conversion, a composite
literal, or a builtin that only reads its arguments.

The body must not change the map or the key, and the loop header counts as part
of the loop. A lookup is also left alone in these cases:

- a key that names a variable of the loop, or one the body changes through a
  field, an element, or a pointer;
- a call other than a conversion or a pure builtin, a channel receive, a
  channel length, a pointer dereference, or an index expression in the key;
- a key that is passed to a call, changed through an alias, or is a
  package-level variable;
- the function holds a pointer to the key's type;
- a key that converts a slice to another type, because a second slice can share
  the backing array;
- a key that the body declares with var or :=; a const key declared in the body
  is reported without a fix;
- the map is assigned, indexed on the left of an assignment, deleted from,
  cleared, passed to a call, used as the receiver of a method, bound to another
  variable, stored in a container, or rebound by a type switch;
- the function calls anything other than a conversion or a builtin that cannot
  reach an existing map or key, sends or receives on a channel, or ranges over
  an iterator or a channel;
- the map or the value that contains it has its address taken, or that
  containing value is copied;
- the body writes another field or element of the value that contains the map
  or the key;
- the map expression is a call, a dereference, an index, or a literal;
- the map expression goes through a pointer, or is declared at package level,
  because the function can change it without naming it;
- the map or key is a local alias of a package-level value;
- the function names another value that can hold a map, because the caller can
  pass both as the same map;
- the function names a value of interface type, which can hold a map or change
  one through a method;
- the function names a function value, which can capture the map or the key;
- the function names a value of a type parameter, because the rule does not
  read the type set;
- the map is bound from a call other than make, or from an expression that is
  not a name, a field path, or a map literal;
- the index expression is itself the target of an assignment or an increment,
  which writes the map;
- the map's key type contains an interface, because hashing an interface value
  can panic;
- the map's type is a type parameter;
- the map's key type is a type parameter, because the rule reads the
  constraint as an interface.

A lookup in a loop's init, condition, post statement, or range expression is
outside that loop's body. It is reported if an outer loop body contains it. A
lookup in a function literal is left alone, since the literal is a separate
function and can run after the loop or not at all. A loop inside the literal
is checked with the enclosing function, which can still withhold it. A lookup
in a function whose name contains benchmark, ignoring case, is left alone,
because a benchmark repeats the lookup to measure it.

The fix hoists the lookup above the loop and replaces the index expression with
the names it binds. A single-result lookup becomes `lookup := m[key]` before
the loop, and the body reads `lookup`. A two-result lookup, as in
`v, ok := m[key]`, becomes `lookup, found := m[key]`, and the body assigns from
`lookup, found`, so the map is read once. The names are `lookup` and `found`,
or the first free names after them. Lookups in one loop that read the same map
and key with the same number of results share one declaration.

The fix is withheld in these cases:

- a comment lies inside the lookup, because the rewrite would delete it;
- a comment lies on the line above the loop, because the declaration would
  separate it from the loop;
- a name used by the map or the key is not in scope above the loop, because the
  declaration cannot name it;
- a `goto` jumps from before the loop to a label on or after it, because Go
  rejects a jump over a variable declaration
  ([Goto statements](https://go.dev/ref/spec#Goto_statements));
- a `goto` after the loop returns to a label on or inside it, because the
  declaration before the loop would not run again;
- the index is a parenthesized right-hand side of a two-result assignment or
  var declaration, because the fix writes the bound names as a list;
- evaluating the key uses a division, a remainder, a shift, a conversion the
  rule cannot prove is array-free, or a comparison with an interface value;
- the declaration or a replaced line would exceed the width `linelen` is set
  to.

A lookup that the fix cannot hoist is still reported.

The rule is adapted from staticcheck's `SA6001`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/maplookupinloop/cmd/maplookupinloop
go tool maplookupinloop ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -maplookupinloop ./...
go tool lesiw.io/tools/cmd/fix -maplookupinloop ./...
```
