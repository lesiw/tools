# lesiw.io/tools/analyzers/rangevaladdress

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/rangevaladdress.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/rangevaladdress)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an address taken from a range loop's
variable and kept past the iteration.

```go
//go:build go1.21

package p

func f(ns []int) (out []*int) {
    for _, n := range ns {
        out = append(out, &n) // address of loop variable n is the same in every iteration
    }
    return out
}
```

Two cases have a shared loop variable. A file that declares a Go version before
1.22 has one, since the spec gives a range clause's variables one instance per
iteration only when the clause declares them: "each iteration has its own new
variables [Go 1.22]" ([For statements with `range`
clause](https://go.dev/ref/spec#For_range)). A range clause that assigns its
variables with `=` takes them from the enclosing scope, so it shares them at
every version. Both are reported. The version is the module's, and a file's
`//go:build` line sets it for that file, in either direction: `//go:build
go1.21` keeps the old behavior in a module that declares Go 1.22.

A file at Go 1.22 or later is skipped, and so is a file whose version is
unknown, unless the range clause assigns its variables. The first case only
arises in a module that has not raised its `go` directive, and raising it
retires every report of that kind at once.

An address taken from the variable is the same address in every iteration, so a
pointer kept past the iteration reads the value the loop wrote last. The key
and the value of a range statement are both loop variables, so an address taken
from either is reported. A body with two nested loops that assign one variable
reports the address in the inner loop, so the copy lands where the fault is.

The rule reports an address where it reaches a target that outlives the
iteration, either directly or through a variable the body assigned it to. A
target outlives it unless the body declares the variable it names, so
`out[i] = &n` and `last = &n` are reported, and `row[0] = &n` with `row`
declared in the body is not, until the body stores `row` where it outlives. A
target the body does not declare is taken to outlive the iteration even when a
call produced it, as `makeSlice()[0]` is. A map target holds the address in its
key, and the map outlives the iteration the same way, so `seen[&n] = true` and
`seen[&n]++` are reported.

The rule reads the expressions that carry an address: the loop variable itself,
a variable the body assigned one to, an element of a composite literal, an
argument of `append` or of a conversion, and the inside of a literal whose own
address is taken, as in `&box{tag: &n}`. A selector or an array index is
followed to the variable, so `&item.tag` and `&item.tags[0]` are reported when
`item` is the loop variable. A field of a struct or an element of an array is
the variable's own memory; one reached through a pointer or a slice refers to
memory the loop does not copy, which each iteration can place elsewhere.

A deferred or spawned call is reported too, since it may read the address after
a later iteration writes the variable. A send into a channel the rule can see
through is reported.

A store is not reported when the next statement after it is a `return`, a
`break` that leaves the loop, or a `goto` past the loop. A `break` leaves the
innermost `for`, `range`, `switch`, or `select` around it, so a `break` under a
`switch` or an inner loop leaves that statement and the store is reported.

Not reported:

- a store into a struct value the body holds, since the value is a copy;
- a store or a send made through a copy of the pointer or the channel, as in
  `p := h` followed by `p.count = &n`, since the rule does not track what the
  pointer refers to;
- a send into a channel the body declares, unless the body stores the channel
  where it outlives the iteration;
- an address reached through a pointer or a slice the variable holds, since
  that memory is not the variable's own;
- an address handed to a call the body makes in place, other than `append`,
  since whether the callee keeps it cannot be seen from the call;
- an address taken inside a function literal the body does not store, which
  `loopclosure` reports when a `go` or `defer` statement starts it; a literal
  called on the spot is reported by neither rule;
- an address a variable is given after it already escaped, since the rule reads
  the body in the order it is written;
- an address taken from a three-clause `for` loop, which this rule does not
  check.

The rule is adapted from revive's `range-val-address`, which reads a range
clause's value variable, does not check the key, and skips a package at Go 1.22
or later.

The report offers an edit that declares a copy of each reported variable as the
body's first statement, as in `n := n` or `k, v := k, v`. The edit is withheld
where the copy would change what the body means, which is a function literal in
the body reading the variable, the body declaring the name itself, or a loop
that assigns its variable and whose body writes it, and where the body's first
statement shares its line with the code before it, which leaves the declaration
no line of its own.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/rangevaladdress/cmd/rangevaladdress
go tool rangevaladdress ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -rangevaladdress ./...
go tool lesiw.io/tools/cmd/fix -rangevaladdress ./...
```
