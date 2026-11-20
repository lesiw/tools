# lesiw.io/tools/analyzers/nilctx

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilctx.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilctx)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a nil argument for a `context.Context`
first parameter.

```go
func fetch(ctx context.Context) {}

func run() {
    fetch(nil) // the context.Context argument is nil
}
```

The context package's doc says: "Do not pass a nil Context, even if a function
permits it. Pass context.TODO if you are unsure about which Context to use."
Calling `Done`, `Err`, `Deadline`, or `Value` on a nil context panics.

The rule reports a call whose first parameter is `context.Context` and whose
argument for that parameter is `nil`. A method expression passes the receiver
as its first argument, and the receiver is not a parameter, so the rule reads
the argument after it:

```go
func (*store) write(ctx context.Context) {}

func run(s *store, ctx context.Context) {
    (*store).write(s, nil) // the context.Context argument is nil
    (*store).write(nil, ctx)
}
```

The rule leaves alone:

- a nil written for a later parameter, as `annotate("", nil)` for
  `func annotate(note string, ctx context.Context) {}`, since only the first
  parameter is read; `ctxfirst` reports a context parameter in another
  position;
- a nil written for a parameter of any other type, including a variadic
  `...context.Context`, whose parameter is a slice;
- a defined type whose underlying type is `context.Context`, and another
  package's interface of that name;
- a type parameter constrained to `context.Context`, since the declared
  parameter type is the type parameter;
- a conversion, as `(func(context.Context))(nil)`, which is not a call;
- a call through a variable, as `handle(nil)` where `handle` is a function
  value, since the rule follows the callee's name to its declaration;
- a variable of type `context.Context`, since the rule reads the argument as
  written.

An alias names the same type, so a parameter declared with
`type trace = context.Context` is reported too.

## The fix

The fix writes `context.TODO()` in place of the nil, under the import name the
file uses, and adds the import where the file has none:

```go
import ctxpkg "context"

func deliver(ctx ctxpkg.Context) {}

func send() {
    deliver(ctxpkg.TODO())
}
```

It is withheld, and the call is still reported, in three cases:

- the file dot-imports or blank-imports `context`, so there is no import name
  to qualify `TODO` with;
- another declaration holds the name the fix would write at that point, as a
  local variable named `context`;
- the rewrite would take the line past the width.

## Provenance

The rule is adapted from staticcheck's `SA1012`. staticcheck offers
`context.TODO` and `context.Background` as two fixes and reads the first
argument of the call; this rule writes `context.TODO`, which is the one the
package doc names, and reads the argument written for the parameter, so a
method expression's nil receiver is left alone.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilctx/cmd/nilctx
go tool nilctx ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilctx ./...
go tool lesiw.io/tools/cmd/fix -nilctx ./...
```
