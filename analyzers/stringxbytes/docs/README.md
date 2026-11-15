# lesiw.io/tools/analyzers/stringxbytes

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/stringxbytes.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/stringxbytes)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a conversion between string and []byte that
nothing needs.

The spec gives `copy` and `append` a string form, and `io.WriteString` calls
`WriteString` in place of `Write`. The call copies the string, because `Write`
may keep the slice; `WriteString` hands the string over.

A fix is withheld when a span it would replace or delete holds a comment.

The rule is like go-critic's `stringXbytes` and `preferStringWriter`.

## Checks

Every check reads a conversion written in place. A conversion stored in a
variable first is left alone, as in `p := []byte(src)` followed by
`copy(dst, p)`. A conversion of or to a type parameter is left alone: the
checks read a type's underlying type, and a type parameter's underlying type is
its constraint.

### A copy or an append that reads the string

```go
copy(dst, []byte(src))      // conversion only to copy; copy takes a string
append(dst, []byte(src)...) // conversion only to append; append takes a string
```

The value being converted can be any type with `string` under its name, and
both forms read its bytes. The conversion has to produce a slice of the
predeclared `byte`: a conversion to a slice of a defined byte type, as in
`octs(s)`, is left alone, because `copy` cannot read a string into one. A
conversion of a byte slice, as in `[]byte(b)`, is left alone because its
operand is not a string. The destination needs no check: a destination that
could not take the conversion would not type-check. `append` without the spread
appends one value, which is a different call. The fix drops the conversion:
`copy(dst, src)` and `append(dst, src...)`.

### An empty string comparison

```go
string(b) == "" // conversion only to compare with an empty string; use len
```

The conversion exists only to test whether `b` is empty, which `len` answers.
`string(b) != ""` becomes `len(b) != 0`.

Four comparisons are left alone: one with the empty string on the left, which
`yoda` reports; one against anything but the empty literal; one ordering
comparison, which has no shorter form; and two conversions compared with each
other, which would need a call to `bytes.Equal` in their place.

The operand has to be a byte slice: `string(65)` yields one rune, not the bytes
of a number. A `switch` tag is not a comparison, so a conversion there is not
read either.

### A conversion taken for its length

```go
len(string(b)) // conversion only to take its length; len reads it directly
```

A conversion between a byte slice and a string copies the bytes and does not
change how many there are, so `len([]byte(s))` is `len(s)` too. The length
check and the comparison both go quiet when `len` names anything but the
predeclared function: the length check reads a call to that function, and the
comparison's fix writes one. The fix is `len(b)`.

### A Write on a receiver that has a WriteString

```go
buf.Write([]byte(src)) // conversion only to call Write; WriteString takes a string
```

The receiver has to reach both methods through the same embedded fields, so a
type's own `Write` does not pair with a `WriteString` promoted from an embedded
field, and two methods promoted from different fields do not pair. An interface
flattens its embedded interfaces, so its `Write` and `WriteString` pair
wherever they are declared. Both signatures have to be the standard pair:
`Write` takes a byte slice, `WriteString` takes a `string`, and the results are
identical. The receivers must match as well: a pointer receiver can
change a value that a value receiver copied.

Five shapes are left alone:

- A named string type as the argument, as in `[]byte(src)` for `type text
  string`: `WriteString` takes a `string`, and `text` needs a conversion to
  reach it.
- A call that does not take exactly one argument; a method expression passes
  the receiver first.
- A variadic `Write`, as in `Write(p ...byte)`: the rename cannot drop the
  spread.
- A receiver with no `WriteString`, which includes a plain `io.Writer`.
- A call from a method of the receiver's own type, which the rename could send
  back to `WriteString`. The method below writes by calling `Write`, so the
  rewrite would call itself:

```go
func (w writer) WriteString(s string) (int, error) {
    return w.Write([]byte(s))
}
```

`Write` and `WriteString` are the only pair `io.WriteString` treats as
interchangeable, so a call to any other method is left alone. The fix is
`buf.WriteString(src)`.

In a test file, a call to a `Write` declared by the test package or by the
package it tests is left alone, since a test or an example of `Write` may be
exercising it. A test
cannot be exercising a conversion the same way, so the other checks still run
there.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/stringxbytes/cmd/stringxbytes
go tool stringxbytes ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -stringxbytes ./...
go tool lesiw.io/tools/cmd/fix -stringxbytes ./...
```
