# lesiw.io/tools/analyzers/modifiesparam

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/modifiesparam.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/modifiesparam)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a dead write to a parameter.

```go
func normalize(name string) {
    name = strings.TrimSpace(name) // dead write to parameter name
}
```

The calls section of the spec says new storage is allocated for a function's
variables, "which includes its parameters and results", and that the arguments
are assigned to them. The caller does not see a write to a parameter, so a
write that no read follows does nothing. A func literal's parameters are
checked the same way.

A parameter the body writes and then reads is a local copy used as a scratch
variable, which the rule leaves alone.

`n += x`, `n++`, `n--`, and the key or value of a range clause are writes too. A
read inside the statement doing the write does not save it, so `n = n + 1` in a
block is reported: it reads the value it replaces. Inside a loop the next pass
is another matter, and the loop rule below saves it.

A read anywhere else in the body after the write saves it. So does a read
anywhere in a loop that contains the write, whether or not the loop runs again,
and a read in any func literal in the body, even one that nothing calls,
because the analyzer cannot tell which literals run. A literal's body is a
function body, so the read its own writing statement makes reads the value it
replaces, as in a block. The analyzer compares
positions, so a read on a branch that cannot follow the write still saves it.

Every dead write is reported, including one that shares its statement with a
target the function reads, as in `n, m = 1, 2` where only `m` is read after it.

A write counts as a use, so `unusedparams` leaves to this rule a parameter that
is only written.

Not reported, and why:

- an assignment to a local variable that shadows a parameter: the rule resolves
  the name, and the write is to the local;
- a write of nil, or of a conversion of nil, because the assignment can end a
  reference's life before the function returns;
- a write through the parameter: a field of a pointer parameter or an element
  of a slice parameter, which the caller does see, and a field or an array
  element of a value parameter, which the caller does not. `unusedwrite`
  reports the field write and the array element write; neither rule reports
  `v.count++`;
- an assignment to a named result: the rule covers the copy the caller passes
  in, even where the write is dead, as in `n = 1; return 2`;
- a write in a body that hands out the parameter's address, by `&` of the
  parameter, of its field, or of an array element, by a slice of an array
  parameter, or by a call to a method with a pointer receiver, since the
  address carries the write to code whose reads the analyzer cannot place;
- a write in a body that contains a `goto`, because the jump can carry a read
  backwards;
- every write before a write that is read, as in `n = 1; n = 2; return n`: the
  rule asks whether any read follows the write, and one does.

## Fixes

An increment, an assignment whose right side is a constant, and a target of a
range clause carry a fix that removes the write. A statement alone on its line
takes the line with it. A post clause becomes part of the header,
`for ; c; n = 1` to `for c`, and a range target goes with the clause that
assigns it.

A write that shares its statement with a value the function reads goes with the
value beside it, `n, m = 1, 2` to `m = 2`, where that value is a constant or a
name used elsewhere. Any other value stays, and the target becomes the blank
identifier, `n, m = pair()` to `_, m = pair()`. A dead value of a range clause
goes with its slot, `for i, n = range xs` to `for i = range xs`, and a dead key
becomes a blank, since dropping it would assign the value where the key was.

A right side that is not a constant is removed by hand: it may do work that
deleting the write would drop.

The fix is withheld where the write shares its line with anything else, as a
clause of an `if`, `switch`, or `for` init does. It is withheld where the write
holds a comment of its own, where a comment starts the line above it, and where
a loop header with an init clause or a comment between its clauses would need a
post slot the fix does not write.

The rule is adapted from revive's `modifies-parameter`, which reports every
assignment to a name in a function declaration's parameter list, without
resolving the name, so a local variable that shadows a parameter is reported
too. revive also reports a parameter passed to `slices.Delete` or
`slices.DeleteFunc`, whose writes reach the caller's array.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/modifiesparam/cmd/modifiesparam
go tool modifiesparam ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -modifiesparam ./...
go tool lesiw.io/tools/cmd/fix -modifiesparam ./...
```
