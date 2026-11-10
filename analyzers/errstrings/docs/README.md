# lesiw.io/tools/analyzers/errstrings

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errstrings.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errstrings)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a capitalized error string or one ending in punctuation.

```go
package a

import (
	"errors"
	"fmt"
)

var errBad = errors.New("Something went wrong") // error string starts with a capital letter

var errStop = errors.New("not found.") // error string ends with punctuation

var errBoth = errors.New("Not found.") // error string starts with a capital letter

func f(n int) error {
	if n < 0 {
		return fmt.Errorf("Negative count: %d", n) // error string starts with a capital letter
	}
	if n == 0 {
		return fmt.Errorf("count is zero") // fine
	}
	return nil
}
```

An error usually appears after other text, as in `reading config: %v`. A
capital letter or a full stop inside the message breaks the sentence the caller
wrote. The check leaves an initialism such as TLS alone.

The idea comes from revive's error-strings.

Report only: the rewrite is a judgment about the words.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errstrings/cmd/errstrings
go tool errstrings ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errstrings ./...
```
