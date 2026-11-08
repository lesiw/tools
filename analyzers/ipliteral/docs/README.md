# lesiw.io/tools/analyzers/ipliteral

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ipliteral.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ipliteral)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an IP address written into source.

An address in a string is an address in one place and a deployment detail
everywhere else. It cannot move to DNS, it cannot fail over, and a reader
cannot tell a test address from a production one. Loopback and the unspecified
address are exempt: they are placeholders rather than locations, and a listener
has to name one.

Only a host is reported, which means a URL, a `host:port`, or a `user@host`. A
bare dotted number is left alone, since it is as likely to be an object
identifier, a version, or a mask as it is to be an address. Test files are
skipped: a fixture names the place it talks to on purpose.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ipliteral/cmd/ipliteral
go tool ipliteral ./...
```
