# lesiw.io/tools/analyzers/packagename

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/packagename.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/packagename)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a meaningless package name, a name that
collides with a common standard library package, a package named `pkg` whose
directory has another base name, a name with an underscore, or a name with a
capital letter.

```go
package util // package name "util" gives no sense of its contents; name the package for what it holds
```

```go
package http // package name "http" collides with standard library package path "net/http"; choose a name that distinguishes this package
```

```go
package pkg // package name "pkg" does not match its directory "config"; the directory already names the package
```

```go
package some_name // package name "some_name" contains an underscore
```

```go
package mixedCaps // package name "mixedCaps" is not all lower case
```

The package name prefixes every exported identifier a client uses, so a name
that gives no sense of the contents is read at every use. Go's
[Package names](https://go.dev/blog/package-names) article says packages named
`util`, `common`, or `misc` "provide clients with no sense of what the package
contains". It also gives the form: lower case, with no `under_scores` or
`mixedCaps`.

The rule reports `common`, `commons`, `helper`, `helpers`, `misc`,
`miscellaneous`, `util`, `utilities`, `utility`, and `utils`, and it reports a
name with an underscore or a capital letter. For the meaningless names, the
whole name is compared, so the rule does not report `httputil`.

The article also says to avoid using the same name as a popular standard
package, such as `io` or `http`. The rule reports a package name that equals one
of `bufio`, `bytes`, `context`, `errors`, `filepath`, `flag`, `fmt`, `http`,
`io`, `ioutil`, `json`, `log`, `math`, `net`, `os`, `reflect`, `regexp`,
`runtime`, `sort`, `strconv`, `strings`, `sync`, `time`, or `url`, and the
report names the import path it shadows. The collision check leaves alone the
standard library package itself, as well as a package under an `internal` or
`vendor` directory, which clients outside its module cannot import.

A package named `pkg` is reported when its directory has another base name. By
convention the last element of the package path is the package name, and the
directory already names the package.

The article also names `api`, `types`, and `interfaces`, and the rule does not
report them: those words name a subject a package holds, as `go/types` holds
the type system and `httputil` holds HTTP helpers, so a check on the name
cannot tell such a package from one that collects everything.

Leaves alone `interface` and `type`: both are Go keywords, so the parser
rejects them as package names.

The report lands on the package name of the first non-generated, non-test file
in the package. A package with no eligible file is not reported. A test file is
skipped because an external test package must be named `pkg_test`, and the
author cannot remove that underscore.

Adapted from revive's `package-naming`.

Report only: a new name would break every package that imports this one, and
the tool cannot choose one.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/packagename/cmd/packagename
go tool packagename ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -packagename ./...
```
