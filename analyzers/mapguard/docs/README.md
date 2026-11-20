# lesiw.io/tools/analyzers/mapguard

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/mapguard.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/mapguard)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a presence check around a map write that
needs none.

```go
func add(index map[string][]string, kind string) {
    if _, ok := index["alpha"]; ok { // map write does not need a presence check
        index["alpha"] = append(index["alpha"], kind)
    } else {
        index["alpha"] = []string{kind}
    }
}
```

The Index expressions section of the spec says a map index yields "the zero
value for the element type of M" when the key is not present
(https://go.dev/ref/spec#Index_expressions). Appending to that zero value
returns the appended values alone, and adding to it returns the summand alone,
so each branch of the check writes the same thing.

The init reads the map index into a blank identifier and the key's presence
into a flag, and the condition is exactly that flag. An inverted check, as in
`if _, ok := index["alpha"]; !ok`, is left alone.

Both branches are present, and each holds one statement. The analyzer reads the
two statements as one of three pairs: an append assignment and a composite
literal holding those values, a `+=` and an assignment of its summand, and a
`++` and an assignment of the constant one.

The `++` pair accepts any constant equal to one, so `0x1` and `1.0` match as
well.

The two writes must go through the same index expression, and the value each
writes must print the same. A value is compared by its printed text: the else
branch's value is dropped by the fix, so a call in a summand is no reason to
refuse. The index is another matter, since the fix reads it a different number
of times than the guard did. It must hold nothing that can give a different key
the second time, so a check on `index[next()]` or `index[len(ch)]` is left
alone.

The fix moves the write out of the if, so the flag named in the init goes out
of scope. A shape whose write reads that flag is not reported.

The rest of the cases are left alone as well:

- a `+=` whose computed summand may be a float or a complex, because a summand
  of -0 adds to +0 where the branch that assigns stores -0;
- an append that spreads a slice, since it appends the slice's elements where
  the literal holds the slice itself;
- an append with no values, since appending nothing to an absent key's nil
  slice leaves it nil where the else branch writes an empty slice;
- a composite literal that names its indexes, since a keyed element matches no
  append argument;
- an init that assigns with `=`, and an else that is a chained if.

The rule is adapted from staticcheck's S1036, which reports the same guard.

Fixable: the if becomes the write in its then branch. A write written on
several lines is written on one line. A comment on its own line above the if
stays above the write.

```go
func add(index map[string][]string, kind string) {
    index["alpha"] = append(index["alpha"], kind)
}
```

An if that is the else branch of another if takes a block around the write,
since else accepts only an if or a block.

```go
} else {
    index["alpha"] = append(index["alpha"], kind)
}
```

The analyzer reports these cases without a fix:

- a comment inside the if, on the if's first or closing line, or between a
  branch and an else-if, where the edit would delete it or leave it describing
  the check;
- a write with no one-line form, as a raw string that contains a newline has
  none;
- a line the fix writes that would run past the width `linelen` is set to,
  unless the guard's first line is already past it; with no width set, every
  line fits.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/mapguard/cmd/mapguard
go tool mapguard ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -mapguard ./...
go tool lesiw.io/tools/cmd/fix -mapguard ./...
```
