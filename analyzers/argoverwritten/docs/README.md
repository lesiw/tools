# lesiw.io/tools/analyzers/argoverwritten

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/argoverwritten.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/argoverwritten)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a function parameter whose passed value is
never used because the body assigns to it.

```go
func reset(count int) int { // the value passed as count is never used
    count = 0
    return count
}
```

The Go spec initializes each parameter variable with the corresponding argument
when the call begins (https://go.dev/ref/spec#Calls), and an assignment
replaces the value stored in the variable
(https://go.dev/ref/spec#Assignment_statements). An assignment to a parameter
before the body uses it replaces the caller's value, so the caller computes and
passes a value that the function never uses.

A parameter is reported when nothing uses the value passed for it and the body
also assigns to it. Any assignment counts, whether in an init clause, a
branch, a loop, or a select case, and an assignment in one branch is enough
when nothing later uses the parameter. A short variable declaration that
redeclares the parameter, as in `x, err := f()`, assigns the existing variable
(https://go.dev/ref/spec#Short_variable_declarations), so it counts too.
Declared functions and function literals are both checked. A use counts when
it appears in the SSA form of the body. A bare `_ = x`, and a switch tag that
is the bare parameter and that no case compares, produce no SSA use, so neither
counts as a use; a use whose value appears in the SSA form, as in `_ = x + 1`,
does count. A temporary whose only use is discarded does not count either, as
in `y := x; _ = y`.

Constant conditions are not evaluated: an assignment under `if false` counts, a
use under one counts as a use, and a condition that is always true is treated
as possibly false. A report can therefore come from an assignment that a
constant condition makes unreachable. Statements after a return, an infinite
loop, a select with no cases, and an expression statement that calls a function
which does not return are not analyzed, and neither is one skipped by a goto,
break, or continue. Reads and assignments use different reachability:
reachability for a use is in the SSA form, and reachability for an assignment
is in the statement control-flow graph. A call that does not return in
another expression position is unreachable to SSA, but the statements after it
still count as reachable to the assignment check. This applies within the
function body the statement is in; a function literal is analyzed even when the
literal itself is unreachable.

Not reported:

- a parameter read before an assignment on some path, including a read in a
  condition or in one branch;
- a parameter assigned in a loop that may run zero times, when the argument can
  reach a read after the loop. A for statement with no condition has no
  zero-iteration path, so an assignment that runs before every exit is
  reported. A range statement is treated as possibly empty, even when its
  operand has a fixed length;
- a parameter that is an incoming value at a control-flow merge, which a loop,
  a jump back to a label, or two branches can produce. The parameter is treated
  as an incoming value of a phi node, which counts as a use, even when every
  later read is of the assigned value;
- a parameter whose address is used, as in `return &x` or a call argument, or
  that a function literal captures. A value is not tracked through a pointer or
  a closure, so an assignment before the address or capture is left alone as
  well; a bare `_ = &x` does not count as a use;
- a parameter that the body never assigns;
- a method receiver, a named result, and a parameter that is blank or unnamed;
- an increment or a compound assignment, which read the old value first;
- a parameter read by the assignment that overwrites it, as in `x = x`,
  `x = x + 1`, or `x, y = y, x`. A dead assignment and its read can disappear
  from the SSA form, so the assignment is also checked for a read. A use stored
  in a blank target, as in `x, _ = 1, x`, does not count.

Report only: the repair names the parameter `_` and declares a local for the
assigned value, drops the assignment, or removes the parameter, and the right
one depends on the caller.

The rule is adapted from staticcheck's SA4009
(https://staticcheck.dev/docs/checks/#SA4009).

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/argoverwritten/cmd/argoverwritten
go tool argoverwritten ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -argoverwritten ./...
```
