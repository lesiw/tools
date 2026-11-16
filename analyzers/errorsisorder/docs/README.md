# lesiw.io/tools/analyzers/errorsisorder

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errorsisorder.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errorsisorder)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to `errors.Is` whose first argument
is a package-level error variable from another package.

```go
func f(err error) bool {
    return errors.Is(errors.ErrUnsupported, err) // first argument is a package-level error; swapping changes behavior
}
```

The errors package documents `Is`
(https://pkg.go.dev/errors#Is): "Is reports whether any error in err's tree
matches target." The first argument is the error being checked and the second
is the error to match. A leaf sentinel with no `Is` method in the first
position makes the call match only the sentinel itself, since it wraps
nothing.

A package-level error variable can also wrap another error or have an `Is`
method. The original call can then be correct, and the swap can reverse its
result. To keep an intentional call, bind the package-level error to a local
variable first; a local first argument is left alone. The suggested fix swaps
the arguments; the diagnostic message is "first argument is a package-level
error; swapping changes behavior".

The first argument has to be a package-level variable from another package.
The variable may be nil or assigned at run time; only its declaration matters.
A first argument that is a local variable, a field, a call, a constant, or a
composite literal is left alone because none is a package-level variable. A
call through a function value is left alone too; only a direct call to
`errors.Is` is matched. A first argument that is a package-level variable from
the package being analyzed is left alone. A call whose two arguments are both
package-level variables from other packages is left alone too: comparing two
package-level errors from other packages is a legitimate use.

The fix swaps the arguments, drops parentheses around either argument, and
swaps the order in which they are evaluated. The fix is withheld when a comment
lies in the argument list, because swapping the arguments can change which one
it describes. The suggested fix message includes the caveat.

The rule is adapted from staticcheck's SA1032, which reports only; this rule
adds the swap.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errorsisorder/cmd/errorsisorder
go tool errorsisorder ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errorsisorder ./...
go tool lesiw.io/tools/cmd/fix -errorsisorder ./...
```
