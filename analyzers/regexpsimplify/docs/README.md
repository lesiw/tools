# lesiw.io/tools/analyzers/regexpsimplify

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/regexpsimplify.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/regexpsimplify)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a character class in a regexp pattern that
has a shorter spelling.

```go
var digits = regexp.MustCompile(`[0-9]`) // [0-9] can be replaced by \d
```

A class of one character matches that character, so `[x]` can be written `x`
and `[a-a]` can be written `a`; a character that cannot stand bare keeps an
escape, so `[.]` is written `\.`. A class of one escape matches that escape, so
`[\n]` is `\n`. A class that matches one of the Perl classes is that class, so
`[0-9]` is `\d`, `[A-Za-z0-9_]` is `\w`, `[[:word:]]` is `\w`, and `[^0-9]` is
`\D`.

The `regexp/syntax` docs give both spellings the same meaning, and both are
correct, so this is a house rule: the fix writes the short one. Go's own
documentation writes `[0-9]` in its examples, and `\d` is ASCII in Go, so a
pattern copied to an engine that reads it as Unicode digits changes meaning.

A class is reported when the short form parses to the same expression as the
class, under the flags the call uses, and the pattern still parses to the same
expression once the rewrite is in place. The second check keeps a rewrite off a
class whose new bytes would join their neighbors: `[\0]1` is left alone
because `\01` is one escape, and `{[1]}` because `{1}` is a repetition. A
case-folded character is compared as the set of characters it folds with, so
`(?i)[k]` can be written `(?i)k`.

`regexp.CompilePOSIX("[0-9]")` is left alone, because `\d` is not POSIX syntax.
`regexp.CompilePOSIX("[x]")` is reported, because a bare character means the
same thing in both syntaxes.

Only a pattern written as a string literal at a `regexp` function that takes one
is checked, because the fix rewrites that literal: `Compile`, `CompilePOSIX`,
`MustCompile`, `MustCompilePOSIX`, `Match`, `MatchReader`, and `MatchString`. A
pattern held in a constant or built by concatenation is left alone, as is a call
through a function value; a call
through a dot import or an aliased import is followed. The argument of a method
on a `*Regexp` is text to match, and is left alone. A pattern that does not
compile is left to `regexpconst`.

The pattern of an interpreted literal is quoted, so `regexp.MustCompile("[0-9]")`
becomes `regexp.MustCompile("\\d")`. A literal that `strconv.Quote` would write
differently is left alone, since writing it back would respell an escape the
change did not touch.

One literal produces one diagnostic, and one fix rewrites every class the
diagnostic names.

A class keeps its brackets when it has no shorter spelling that matches the same
strings:

- `[[:space:]]` and `[0-8]`, because no Perl class matches those characters:
  `\s` omits `\v`, and `\d` takes `9` as well.
- `[ ]`, because the fix does not write a bare space, which is easy to lose,
  and a class holding a non-ASCII character, because the fix does not write one
  bare: `[é]` keeps its brackets, and `[\x{100}]` is reported.
- `[^a]`, because a negation the rule cannot spell shorter keeps its brackets;
  `[^\p{L}]` is `\P{L}` and is reported.

The rule is inspired by go-critic's regexpSimplify, which upstream tags
opinionated and experimental. It matches a fixed list of spellings and also
rewrites repetitions, alternations, and ranges; this one compares the parsed
sets.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/regexpsimplify/cmd/regexpsimplify
go tool regexpsimplify ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -regexpsimplify ./...
go tool lesiw.io/tools/cmd/fix -regexpsimplify ./...
```
