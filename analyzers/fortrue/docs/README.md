# lesiw.io/tools/analyzers/fortrue

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/fortrue.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/fortrue)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a for statement whose condition is the
literal true and whose for clause has no init or post statement.

```go
func f() {
    for true { // loop condition is true; drop it
    }
}
```

The For statements section of the spec says the condition may be omitted, in
which case it is equivalent to true
(https://go.dev/ref/spec#For_statements). A loop whose condition is written as
`true` runs the same way as the bare loop.

```go
for {
}
```

A loop is not reported when:

- its for clause has an init or post statement, as in `for i := 0; true; i++`;
  the fix replaces the whole header, which would remove that statement;
- the name `true` is not the predeclared constant, as when another declaration
  shadows it;
- the condition is a named constant whose value is true, because the rule
  reports only the predeclared `true`;
- a comment sits in the loop header, because the fix replaces the whole header.

The rule is adapted from staticcheck's S1006
(https://staticcheck.dev/docs/checks/#S1006), narrowed to the predeclared
`true`. S1006 also reports a named constant whose value is true.

Fixable: everything between `for` and `{` is replaced, including the
condition and its parentheses.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/fortrue/cmd/fortrue
go tool fortrue ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -fortrue ./...
go tool lesiw.io/tools/cmd/fix -fortrue ./...
```
