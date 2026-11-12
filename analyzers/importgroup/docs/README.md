# lesiw.io/tools/analyzers/importgroup

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/importgroup.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/importgroup)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` for import grouping.

A block has three groups, in this order: the standard library, other
modules, and the packages the `-local` flag names. One blank line
separates them, and each group is sorted by path. `goimports` sorts within a
group and separates the standard library from the rest, so a block can be
grouped in many ways and still compile and format.

## Usage

    go tool lesiw.io/tools/cmd/vet -importgroup ./...
    go tool lesiw.io/tools/cmd/fix -importgroup ./...

`-importgroup.local` names the prefixes whose packages take the last
group. The flag is comma-separated, and every prefix in it shares one
group. A path matches a prefix at a path segment, so give a whole path
element rather than a fragment, and a standard library path stays in the
first group whatever the flag says.

    go tool lesiw.io/tools/cmd/vet \
        -importgroup.local=lesiw.io,labs.lesiw.io ./...

## Checks

### A block that is not grouped

The diagnostic anchors at the first spec of the block, and the fix
rewrites the block's specs into the three groups, keeping each spec's
comment.

A block holding a comment that no spec owns is left alone: the
comment is part of the group it sits in, and there is no way to say
where it goes.
