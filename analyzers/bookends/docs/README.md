# lesiw.io/tools/analyzers/bookends

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/bookends.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/bookends)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` for bracket layout. Five shapes are reported, and each
fix moves a bracket or a line break; no element is added or removed:

- a closing bracket that is not at the indent of its opening bracket;
- an elided-type literal whose opening brace shares a line with its first
  element, so its elements start at different columns;
- a table whose rows are not all one shape, where one row is on a line and
  another spans lines, which is neither a table on one line nor a table to
  read down;
- a list indented across two lines to carry one line of elements, and a list
  with a line past the width;
- a keyed literal on one line past the width, which takes one key per line.

For a list, the layout with the least raggedness is chosen, so elements spread
across lines by width. Two indented lines within the width are a grouping the
author chose, and the rule leaves them alone. An element written across lines
keeps its text and takes a line of its own. A pair inside an `if`, `for`,
`range`, or `switch` header is left alone. The list keeps one element per line
when its elements are keyed, when a literal's elements are its rows or its
constants, and when a comment sits among them. A `slog` call is the exception:
its trailing arguments are key-value pairs, so each key keeps the value it
labels.

## Usage

    go tool lesiw.io/tools/cmd/vet -bookends ./...
    go tool lesiw.io/tools/cmd/fix -bookends ./...

## Checks

### A closer at the wrong indent

A closer on a line of its own is indented to match its opener. A closer
that trails the last item of a list keeps that item's indent instead, so
the two ends of one pair land at different columns.

    xs = append(xs, T{a: 1,
        b: 2}) // closer is not at the indent of its opener

The fix starts a new line before the closer and adds the comma that line
needs. The printer then places the closer under its opener.

- Only a pair that takes a trailing comma is reported. A closer on its own
  line must follow a comma: any other token at the end of a line takes a
  semicolon with it, so an index, a slice, a paren and a type assertion
  cannot break before their closer at all.
- A closer that already starts a line is left alone. The printer places it
  at the opener's indent whatever the source says, so the pair is already
  balanced.
- A parameter list is left alone. A signature stays on one line or wraps
  as its author chose, the same exemption the width check makes.
- A comment between the last item and the closer stays where the author put
  it. The comma goes in front of the comment and the new line goes after it.

### An opener that shares its line

A literal whose type is elided shares its first line with its opening
brace, so its elements start at different columns:

    rows = []row{{1, 2, 3, 4,
        5, 6, 7, 8}}

The fix moves the first element to a line of its own, and every element
then starts at one column.

### A table whose rows are not all one shape

A row on a line and a row across lines in the same literal is neither a
table on one line nor a table to read down. The rebuild writes the whole
literal as one edit: every row on its own lines, and each row's braces on
the lines its elements are on.

### A list indented to carry one line

Indenting a list across two lines to carry one line of elements wastes the
indentation. The fix packs the elements by the layout with the least
raggedness, the sum of the squared space left at the end of each line. Two or
more indented lines are a grouping, so they stay, unless the list is a `slog`
call's key-value pairs or holds a line past the width. A list whose every line
would hold one element written across lines is left alone: the line holds
nothing but an opener, so there is no layout to offer.

### A keyed literal past the width

A keyed literal on one line that runs past the width takes one key per
line, which is the shape it takes once it no longer fits.
