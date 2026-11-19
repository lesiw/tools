# lesiw.io/tools/analyzers/unwrap

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unwrap.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unwrap)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` for a construct that is wrapped across lines but
belongs on one.

## Usage

    go tool lesiw.io/tools/cmd/vet -unwrap ./...
    go tool lesiw.io/tools/cmd/fix -unwrap ./...

## Checks

### A wrapped construct

A call, binary expression, control-flow header, single-spec declaration block
or interface type that spans lines is reported when the line it would produce
fits:

    g(a, // a wrapped construct belongs on one line
        b)

The fix joins the lines. The width is analyzers/linelen; with none set, every
construct that can be joined is joined.

An interface type goes back only where go/printer prints it on one line, which
takes at most one short field:

    func narrow(err error) error {
        h, ok := err.(interface {
            Hint() error
        })
        if !ok {
            return err
        }
        return h.Hint()
    }

A long method is not joined at any width, an interface holding two methods has
no one-line form at all, and a `//` comment inside one keeps its lines, because
joining would swallow the comment.

An interface inside a signature or a header goes back only with the construct
around it: a signature joins at any width, and a header when the whole of it
fits.

An interface join inside a declaration block, a struct type, or a literal also
has to leave the lines around it within the width: the printer pads those lines
to their widest entry, so a join in one moves the columns of the rest.

A type declaration's interface type is left alone, and so is an alias's. The
braces the declaration was written with stay, though a wrapped method signature
inside one is still joined:

    type declared interface {
        Hint() error
    }

A type parameter's constraint is a use of an interface type rather than a
definition, so it is reported:

    type set[T interface {
        Size() int
    }] struct{ v T }

A function type is a signature wherever it appears, and it is reported
whatever the width of the line it lands on:

    handle := func(
        first *Alpha, second *Beta,
        third map[string]*Gamma, fourth map[string][]int,
    ) int {
        return 0
    }

That joins to 102 columns and stays. A signature holding a `//` comment is
left as it is, because joining it would swallow the comment.

A bracketed list the author broke into two or more indented lines is left
alone: the grouping is written nowhere but the layout, and joining it would
erase the grouping.
