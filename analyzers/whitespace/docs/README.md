# lesiw.io/tools/analyzers/whitespace

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/whitespace.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/whitespace)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` for the blank lines around declarations.

## Usage

    go tool lesiw.io/tools/cmd/vet -whitespace ./...
    go tool lesiw.io/tools/cmd/fix -whitespace ./...

## Checks

### A missing blank line

A multi-line top-level declaration needs a blank line between it and each
neighbor, and a doc comment inside a parenthesized block or a struct field
list needs one above it:

    func b() {
        println(1)
    }
    func c() {} // a declaration needs a blank line above it

The fix inserts the blank line.
