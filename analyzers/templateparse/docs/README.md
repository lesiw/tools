# lesiw.io/tools/analyzers/templateparse

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/templateparse.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/templateparse)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a template that does not parse.

```go
func page() *template.Template {
	return template.Must(template.New("page").Parse(`{{.Kind}`)) // template does not parse: bad character U+007D '}'
}
```

`Parse` returns the template and an error, and `template.Must` panics on a
non-nil one, so the constant above fails on every run. An `html/template` call
is reported the same way, and a template that is deliberately invalid, as in a
test of a parse error, is reported too.

## Delimiters

The rule follows a template from the `New` call that builds it, through the
methods that return a template, so the delimiters `Delims` sets along the way
are the ones the text is parsed with:

```go
func page() *template.Template {
	return template.Must(template.New("page").Delims("[[", "]]").Parse(`[[.Kind]`)) // template does not parse: bad character U+005D ']'
}
```

Only a constant text is checked. A text held in a variable or a parameter is
left alone, and so is a template held in a variable or a field, whose
delimiters may have been set where the call cannot be seen.

## Functions

A call to a function the template does not define is reported, unless the
template was built with `Funcs`, whose map the analyzer does not read.

The rule is adapted from staticcheck's SA1001
(https://staticcheck.dev/docs/checks/#SA1001), which reports a call to `Parse`
on the result of `New`. This rule reads the delimiters that `Delims` sets, and
reports the parse errors upstream leaves alone.

Report only. Making a template parse needs the text its author meant, which the
analyzer cannot supply.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/templateparse/cmd/templateparse
go tool templateparse ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -templateparse ./...
```
