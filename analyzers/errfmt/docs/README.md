# lesiw.io/tools/analyzers/errfmt

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errfmt.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errfmt)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` for error format strings: error
construction routes through `fmt.Errorf`, and error wrapping
routes through its `%w` verb.

`errors.New` can neither format nor wrap, and `fmt.Errorf`
does both. A file that imports `fmt` and uses nothing else in `errors`
keeps one error constructor where it had two.

## Checks

### errors.New can be replaced by fmt.Errorf

The check fires where the rewrite adds no import and leaves none unused:
every use of `errors` in the file is a call to `errors.New` whose message is a
literal, and `errors` and `fmt` are imported by name, with no declaration
shadowing `fmt` at any call. One call the rewrite cannot make leaves the whole
file alone, since the `errors` import cannot go while that call remains.

```go
package store

import (
    "errors"
    "fmt"
)

var errMissing = errors.New("missing item") // errors.New can be replaced by fmt.Errorf

func open(name string) error {
    return fmt.Errorf("open %s: %w", name, errMissing)
}
```

Silent, deliberately: the rewrite would add an import.

```go
package store

import "errors"

var errInvalid = errors.New("invalid item")
```

Silent, deliberately: the message is not a literal, and the file's literal
message goes with it.

```go
package store

import (
    "errors"
    "fmt"
)

const message = "invalid item"

var errClosed = errors.New("closed item")

func open(name string) error {
    if name == "" {
        return errors.New(message)
    }
    return errors.New(fmt.Sprintf("cannot open %s", name))
}
```

The check reports only a message written at the call as a single string
literal. A named constant is left alone, since escaping the message it shares
would change every call that reads it, and a concatenation is not folded into
one. A variable is left alone as well: `go vet` rejects its rewrite in a file
that declares Go 1.24 or later.

Reported only: a percent sign in the message would become a verb, so the
rewrite can change what the error says.

### Errors wrap with %w

An error value formatted into another error can be wrapped: `%v`
and `%s` verbs with error arguments become `%w`.

```go
fmt.Errorf("open failed: %v", err) // error can be wrapped with %w, not %v
```

The fix rewrites the verb in place, and is withheld when the
format's source spelling differs from its value, as with escape
sequences. Only arguments whose static type is `error` are
considered, and only literal format strings are inspected. An
indexed or star verb makes the argument mapping non-positional and
skips the whole call.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errfmt/cmd/errfmt
go tool errfmt ./...
```
