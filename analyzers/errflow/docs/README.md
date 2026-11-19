# lesiw.io/tools/analyzers/errflow

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errflow.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errflow)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a function which writes the same error path
two ways.

A step is one call and the guard that checks its error. Go keeps the normal path
at the function's own level and indents the error handling, so a function that
guards one step guards the next one the same way. Three shapes break that.

## Checks

### An else block after a block that exits

```go
func returning(err error) int {
    if err != nil {
        return 0
    } else { // unnecessary else
        return 1
    }
}
```

The if block ends in a return, so the else runs only when the condition is
false, and its code could run at the function's own level. The indent adds
nothing.

An else-if chain and an if with an init statement are left alone.

### A guard that only returns what it tested

```go
func oneStep() error {
    if err := load(); err != nil { // redundant guard; write return load()
        return err
    }
    return nil
}
```

The guard returns the value it tested and the next statement returns nil, so the
pair is one return written twice. It collapses into `return load()`.

A step written on two statements collapses the same way. The statement that
produces the value goes with the guard, and the return replaces both:

```go
func declared() error {
    err := load() // redundant guard; write return load()
    if err != nil {
        return err
    }
    return nil
}
```

A `var` block holding more than one spec is not the step, because replacing the
block would delete its other specs. The block stays and the return names the
value.

The collapse leaves alone a guard whose condition uses `==`, whose body returns a
different value or holds more than one statement, or that has an else branch. It
also leaves alone a declaration of more than one name or value, a value assigned
rather than declared, and a value an earlier statement declares or assigns,
unless that statement is a `var` block of more than one spec. A concrete value
returned as an error is left alone too, because a nil value of the concrete type
becomes a non-nil error.

### A last call that takes no guard

```go
func twoSteps() error {
    if err := load(); err != nil {
        return err
    }
    return save() // unguarded last call; write if err := save(); err != nil { return err }
}
```

The first step guards its error and the last one returns it unchecked, so the
two steps read differently. The fix writes the guard the step above uses, then
returns nil:

```go
func twoSteps() error {
    if err := load(); err != nil {
        return err
    }
    if err := save(); err != nil {
        return err
    }
    return nil
}
```

The expansion leaves alone a last statement that is not a call, a call that
returns another type, a conversion to `error`, and a call to `errors.New` or
`fmt.Errorf`: a conversion is not a call, and the guard around those two
constructors would always take its branch. A constructor the rule cannot name is
reported, since it may return nil, and an author whose own constructor never does
writes `//ignore:errflow`. The check reads the function's own body, so a guard
and a call inside a nested block are left alone.

### Both shapes

One list takes at most one of the two shapes, so no function reaches both fixes.
The expansion adds a guard, which leaves the list with two nil checks and the
collapse with a condition it does not meet. The collapse removes the list's only
nil check, which leaves the expansion no guard to copy.

Both shapes apply only when the function's only result is `error`. A `(T, error)`
result is left alone, because the guard would need a zero value for `T`.

## Fixes

The collapse writes the expression as the author wrote it, and returns the
expression the if's init statement declares. It writes a comment between the
guard's condition and its closing brace above the return, at the return's indent,
and leaves a comment inside the expression where it is. It is withheld when it
would delete any other comment, and when the nil return shares its line with one.

The expansion writes a comment above the guard when the fix would delete it, and
leaves one inside the call where it is. A comment after the return is written
there when the return's line holds nothing after it but that comment and no
comment begins on the next line: text after the comment would be left behind, and
a comment below it would be separated from it.

Either fix is withheld when a comment it moves is a directive, because a tool
reads a directive where it is written, and when a comment it moves begins
mid-line and covers more than one line, because the fix cannot reindent it. A
directive inside the expression or the call is kept, because the fix writes that
text as the author wrote it. Both shapes report even when the fix is withheld.

The expansion reads the tab run that begins the line it replaces and writes one
tab past it for the guard body, and it is withheld when the line begins with
anything else. The collapse reads that run only to write a comment above the
return, so a line indented another way still collapses when it holds no comment.

The expansion puts the call on the header line, so a long call may make a line
that `linelen` reports, and a call written across lines makes a header that
`linewrap` reports. Hoist the call into a variable above the guard, and wrap its
arguments there. The collapse writes `return` where the statement that produced
the value began, so its first line is at most two columns wider than the one it
replaces, and a line near `linelen`'s limit may go past it.

The else rule reports only; `ifreturn` carries the fix for the body that only
returns.

The else rule is inspired by revive's `indent-error-flow`. The guard that returns
the value it tested is adapted from revive's `if-return`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errflow/cmd/errflow
go tool errflow ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errflow ./...
go tool lesiw.io/tools/cmd/fix -errflow ./...
```
