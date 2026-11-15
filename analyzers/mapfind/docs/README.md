# lesiw.io/tools/analyzers/mapfind

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/mapfind.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/mapfind)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a loop that searches a map for one key.

```go
func f(m map[string]int, want string) (int, bool) {
    for k, v := range m { // loop can be replaced by a map lookup
        if k == want {
            return v, true
        }
    }
    return 0, false
}
```

A map is a hash table, and its index expression yields the element for a key
together with whether the key is present. A lookup therefore reaches the entry
directly, where a range visits every key.

The loop ranges over a map, with or without the value, and holds one statement
in its body. That statement is an if whose condition is a single `==`
comparison of the key with one expression, and whose body is one return.
The loop must declare the key with `:=`, because a loop that assigns to a key
declared outside it leaves that key holding the last key visited, which the
rewrite drops. A loop that negates the comparison, as in
`if k != want { return v, true }`, is left alone.

The wanted key must not name the loop's key or value, and evaluating it must
not panic: a range evaluates it for each entry it visits, and the lookup
evaluates it once. These are left alone:

- a call, which can run code, or a channel receive; `len`, `cap`, `min` and
  `max` are allowed, with their arguments held to the same rules;
- a selection through a pointer, promoted or not, a dereference, a slice
  expression, or an index into an array, a slice, or a string, which can panic
  on a nil pointer or an out-of-range index;
- a division, a remainder, or a shift, which panic on a zero divisor or a
  negative count;
- a comparison of two interface values, which panics when both hold the same
  type and that type cannot be hashed;
- a type assertion, which panics when the value does not hold the type;
- a conversion that builds an array, which panics on a slice that is too short.

The key must also be assignable to the map's key type: a comparison accepts a
value of interface type, and an index rejects one.

An if with an init or an else is left alone: the new header cannot take a
second init, and the else would be lost.

A map whose key type holds an interface is left alone, because an index panics
on a value that cannot be hashed where a comparison is merely false. A type
parameter has an interface for its constraint, so a map keyed by a type
parameter, or a range over one, is left alone even where the constraint
declares only concrete types.

The key may appear in the return only as an index of that same map: the map has
to be a name, a field, an index, or a dereference, so that the return and the
loop reach it the same way. A return that contains a function literal
that reads the key is left alone: nothing binds the key after the rewrite, and
a lookup inside the literal would read the map when the literal runs.

The rule is adapted from revive's `inefficient-map-lookup`, which reports the
comparison without reading the if body and offers no fix.

Fixable: the loop becomes an if that indexes the map in its init, as in
`if v, ok := m[want]; ok`, and the block keeps the return. The init binds the
flag always, and the value when the return reads it. The rewrite picks names
that nothing in scope at the loop declares, so it shadows nothing. A loop that
declared a value keeps that name, and a lookup in the return becomes the name
the init binds, so `return m[k], v` becomes `return v, v`.

The report offers no fix in five cases:

- a comment in the text the rewrite deletes, because the comment belongs to the
  author;
- a line it writes that would run past the width `linelen` is set to, because
  the wrap is the author's to place; with no width set, nothing overflows;
- a map or a wanted key with no one-line form, as a raw string has none,
  because the rewrite writes one line;
- a call or a channel receive in the return before a lookup, because the
  rewrite would run it after the read; indexing the map again in the return, as
  in `if _, ok := m[want]; ok { return count(m) + m[want] }`, keeps the order;
- a value name that equals the map expression, because the result reads as a
  shadow of it.

A comment that trails the loop's first line stays at the end of the new if,
except when the loop and its body share one line, where it ends up after the
closing brace. A comment that trails the if's first line moves above the
return.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/mapfind/cmd/mapfind
go tool mapfind ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -mapfind ./...
go tool lesiw.io/tools/cmd/fix -mapfind ./...
```
