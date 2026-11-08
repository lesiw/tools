# lesiw.io/tools/analyzers/unwrap

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unwrap.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unwrap)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` for a construct that is wrapped across lines but fits
on one.

## Usage

    go tool lesiw.io/tools/cmd/vet -unwrap ./...
    go tool lesiw.io/tools/cmd/fix -unwrap ./...

## Checks

### A wrapped construct that fits

A signature, call, binary expression, control-flow header or single-spec
declaration block that spans lines is reported when the line it would produce
fits:

    g(a, // a wrapped construct fits on one line
        b)

The fix joins the lines. The width is analyzers/linelen; with none set, every
construct that can be joined is joined.
