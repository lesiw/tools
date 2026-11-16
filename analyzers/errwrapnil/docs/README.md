# lesiw.io/tools/analyzers/errwrapnil

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errwrapnil.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errwrapnil)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `fmt.Errorf` call whose `%w` verb has a
nil error operand.

```go
func load() error {
    return fmt.Errorf("fetch: %w", nil) // fmt.Errorf has a nil %w operand
}
```

The nil check is adapted from `nilerrwrap`, which is inspired by the x/tools
printf analyzer. `nilerrwrap` reports the nil operand in positional calls; this
analyzer adds the fix and reports indexed and star calls. Because the nil case
overlaps with a report from `nilerrwrap`, a call that both analyzers report has
two diagnostics.

The `fmt.Errorf` documentation says a `%w` verb with an error operand returns
an error whose `Unwrap` method returns that operand. With one `%w` verb, a nil
operand makes that `Unwrap` return nil, so the operand does not contribute a
cause, and the message contains `%!w(<nil>)` where the cause would be. With several
`%w` verbs, the error implements `Unwrap() []error` and the nil operand is
omitted from the slice.

The fix is offered when the operand count matches the verb count and the
number of non-nil `%w` verbs that would remain is not one. It edits a literal whose source is its value; any other constant
expression is replaced with a quoted literal. The edit removes the verb, its
operand, and the separating comma; the surrounding message text stays. A fix is
withheld when an edit would remove a comment or leave a comment separated from
the code it describes. Applying the fix can leave an import unused; cmd/fix
runs goimports after the edits, and another driver may not. A fix is also
withheld when the format expression or a removed operand contains a dot-imported
identifier, because goimports does not remove a dot import.

If no non-nil `%w` verb remains, the returned error does not have an `Unwrap`
method; this is the single-`%w` case and the all-nil case. `errors.Unwrap`
returns nil before and after. When every `%w` verb is removed, the result of a
type assertion or `errors.As` to an interface with `Unwrap` changes, because
the returned error does not have an `Unwrap` method. Code that asserted the
method must be updated. Removing the method is the intended effect. If at least
two non-nil `%w` verbs remain, the analyzer offers the fix; the returned error
has the same errors in the `Unwrap() []error` slice. If exactly one non-nil
`%w` verb remains, the call is reported without a fix, because the method
changes from `Unwrap() []error` to `Unwrap() error`.

A nil operand is the predeclared nil or a conversion of nil to an interface
type. Any other expression is not inspected for nilness; a nil variable is left
alone because the analyzer does not follow dataflow. A typed nil pointer
that implements error is left alone: the interface value is non-nil, so
`Unwrap` returns it. A typed nil pointer that does not implement error is not a
nil error operand; `nilerrwrap` reports it in the suite. A type parameter is
left alone; the analyzer does not inspect its type set. A call through a
function value is left alone, because the callee expression is not `fmt.Errorf`.

The format has to be a constant string. The operand positions of a spread call
are unknown, so it is left alone. A call with an indexed argument or a star
width or precision that resolves to the nil `%w` operand is reported without a
fix. The analyzer does not model the remaining index and star positions, and a
star operand can be an expression with side effects. A valid index or star is
mapped to the operand position. For a missing `%w` argument, `nilerrwrap`
reports positional calls in the suite; neither analyzer reports indexed or star
calls whose `%w` operand is missing. A call with a nil `%w` operand whose
operand count does not match the verbs is reported without a fix, because the
operand count would still differ from the verb count. A directive that cannot
be parsed ends the scan; a nil `%w` before it is reported without a fix.

In a test file, the analyzer reports the nil operand without a fix: a test may
deliberately check the nil marker, and the fix is withheld conservatively. In
the suite, `nilerrwrap` also reports it.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errwrapnil/cmd/errwrapnil
go tool errwrapnil ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errwrapnil ./...
go tool lesiw.io/tools/cmd/fix -errwrapnil ./...
```
