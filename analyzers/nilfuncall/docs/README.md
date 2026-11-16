# lesiw.io/tools/analyzers/nilfuncall

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilfuncall.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilfuncall)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call of a function value that the same
block set to nil.

```go
func f() {
    h := func() {}
    h = nil
    h() // function value is nil; the call panics
}
```

The specification says "Calling a nil function value causes a run-time panic"
([Calls](https://go.dev/ref/spec#Calls)). The block sets the variable to nil,
and no statement between the assignment and the call writes the variable again.

The walk reads the statements of a block once, in source order. A block's nil
set does not cross a block boundary: an assignment in an enclosing block does
not reach a nested one, and a nested block's assignment does not reach the
enclosing one. A call or an assignment in the init clause, condition, tag,
post, or range expression of a control statement is left alone, and so is a
call in a case or communication clause header. A call in an earlier statement
than the assignment is left alone as well.

A parameter, a receiver, a result, and a field are left alone, and so is a
package-level variable: a field is written through any pointer to the struct,
and another goroutine can write a package-level variable between the assignment
and the call.

A variable is left alone when the function takes its address, since the pointer
can be written through, and when a function literal other than the one holding
the block writes it. A copy of the variable is a different variable, and is left
alone. A function that holds a goto is left alone: the assignment and the call
need not run in the order they are written.

Only an assignment of the predeclared nil counts: a declaration, whether or
not it writes nil, and a conversion of nil such as `hook(nil)` are left alone.

The suite's nilness pass reports most of the same calls from SSA. It does not
read the body of a package-level function literal, and loses the value in a
literal that captures the variable, so those two shapes are this rule's own;
the rest are reported twice.

Report only: dropping the assignment makes the call run the function the
variable held before, and dropping the call drops the work.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilfuncall/cmd/nilfuncall
go tool nilfuncall ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilfuncall ./...
```
