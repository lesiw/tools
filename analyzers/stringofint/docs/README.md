# lesiw.io/tools/analyzers/stringofint

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/stringofint.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/stringofint)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a conversion of an integer to a string.

```go
func f(n int) string {
	return string(n) // string(n) can be replaced by strconv.Itoa(n)
}
```

The spec says what the conversion does: "Converting a signed or unsigned
integer value to a string type yields a string containing the UTF-8
representation of the integer. Values outside the range of valid Unicode code
points are converted to `"\uFFFD"`." The decimal digits of the number are what
the conversion almost always means. An author who wants the character writes
`string(rune(n))`, which says so and is not reported; a value that may not fit
a rune needs `fmt.Sprintf("%c", n)`, since `rune(n)` truncates where
`string(n)` does not. The compiler rejects the conversion in a file that names
Go 1.28 or later (https://go.dev/issue/3939).

A byte and a rune are left alone: an operand whose underlying type is `uint8`
or `int32` is exempt, whatever the type is called. `string(b)` and `string(r)`
read as a single character, which is what a program writing a character
means. A conversion of an untyped rune constant, `string('a')`, is exempt too.
x/tools' `stringintconv` makes the same exemptions.

A conversion is reported where the target's type set holds a string and every
type in the operand's set is an integer, at least one of them a number. A type
parameter is read through its whole set: a set that mixes an integer with a
string or a slice converts only some of its instantiations, so the rule leaves
it alone.

The fix writes the digits as a `strconv` call, which changes the output from
one character to its digits. `strconv` is written rather than
`fmt.Sprintf("%d", n)` because Sprintf calls a `Format` method that a named
integer type may declare, and the digits are wanted whatever the type
declares.

`strconv.Itoa` takes an `int`, so a value of another type is converted first:
`int8` and `int16` to `int`, which holds either on every platform, and the rest
to `int64` or `uint64`, which loses nothing. `int` is 32 bits on some
platforms, so an untyped constant outside the range of `int` takes
`strconv.FormatInt`. A missing import of `strconv` is added as a declaration
after the package clause. `cmd/fix` folds that into the file's own block.

## When the fix is withheld

- No one call writes the value: the operand's set mixes signed and unsigned
  integers, mixes a number with a byte or a rune, or holds a constant outside
  the range of the type the call takes.
- The target's set holds a type that is not a string, so the conversion cannot
  keep its name around a call.
- Go requires the conversion to be a constant: it is the value of a constant
  declaration, the length of an array type, or the index of an array or slice
  literal. A call is not a constant.
- A name the rewrite writes reaches another object at the conversion, either
  `strconv` or the `int`, `int64`, or `uint64` the value is converted to.
- The file binds `strconv` to the dot or blank name.
- A comment sits in the text the rewrite replaces.

The rule is adapted from x/tools' `stringintconv` and revive's `string-of-int`.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/stringofint/cmd/stringofint
go tool stringofint ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -stringofint ./...
go tool lesiw.io/tools/cmd/fix -stringofint ./...
```
