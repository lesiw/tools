# lesiw.io/tools/analyzers/regexpmatchloop

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/regexpmatchloop.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/regexpmatchloop)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to `regexp.Match`,
`regexp.MatchString` or `regexp.MatchReader` in a loop body, where the pattern
is a constant.

```go
func f(ss []string) {
    for _, s := range ss {
        regexp.MatchString("a+", s) // regexp.MatchString compiles the pattern on each call; compile it once before the loop
    }
}
```

The `regexp` documentation for these functions says: "More complicated queries
need to use [Compile](https://pkg.go.dev/regexp#Compile) and the full
[Regexp](https://pkg.go.dev/regexp#Regexp) interface." Each function calls
`Compile` before it matches
([regexp.go](https://go.dev/src/regexp/regexp.go)). Each call compiles its
pattern first, and a constant pattern compiles the same way, so one compile
before the loop suffices.

Only a constant pattern is reported. A pattern held in a variable, a parameter,
or a field can change between iterations. A call through a dot import or a
function value is not a package-qualified call and is left alone. A function
literal is a separate function: a call it holds is left alone, and a loop
inside the literal is checked there. A call in a loop's init, condition, post
statement, or range expression is outside that loop's body; it is reported if
an outer loop body contains it.

The fix hoists `re := regexp.MustCompile(pattern)` above the loop and replaces
the call with `re.MatchString(s)`, `re.Match(b)`, or `re.MatchReader(r)`. The
variable is named `re`, or the first free name after it, and each fixable call
gets a separate variable. The package name is the one the file gives `regexp`,
so an alias is kept. The fix is withheld in these cases:

- the pattern does not compile, because `MustCompile` would panic where the
  call returned an error; `regexpconst` reports the pattern as well;
- the call is not an expression statement, a `go` statement, or a `defer`
  statement, as in `matched, err := regexp.MatchString(pattern, s)`, because
  the method returns one result;
- a comment sits in the call's text or immediately before the loop, because
  the comment would be deleted or left apart from the loop;
- a name in the pattern is not in scope above the loop, as with a constant
  declared inside the loop body;
- a goto in the function jumps from before the loop to a label after the
  declaration, because a goto may not jump over a variable declaration
  ([Go spec](https://go.dev/ref/spec#Goto_statements)).

The rule is adapted from staticcheck's `SA6000`, which suggests `Compile`
instead of `MustCompile`.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/regexpmatchloop/cmd/regexpmatchloop
go tool regexpmatchloop ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -regexpmatchloop ./...
go tool lesiw.io/tools/cmd/fix -regexpmatchloop ./...
```
