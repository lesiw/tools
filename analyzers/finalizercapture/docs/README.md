# lesiw.io/tools/analyzers/finalizercapture

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/finalizercapture.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/finalizercapture)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `runtime.SetFinalizer` call whose
finalizer captures the object through its own variable or bound receiver.

```go
func f() {
    v := new(int)
    runtime.SetFinalizer(v, func(*int) {
        println(v) // finalizer captures the object it finalizes
    })
}
```

The `runtime.SetFinalizer` documentation describes the cycle: "If a cyclic
structure includes a block with a finalizer, that cycle is not guaranteed to be
garbage collected and the finalizer is not guaranteed to run"
(https://pkg.go.dev/runtime#SetFinalizer). The object is reachable from the
finalizer, so it may not be collected and the finalizer may not run.

The rule reports a call whose second argument is a function literal or method
value that captures the first argument. A function literal stored in a local
variable is followed to the call. A method value is reported
when it is written at the call, its receiver expression is the object,
including a field selector such as `h.it`, and the method has a pointer
receiver. A stored method value, a promoted method, and a method value with a
value receiver are left alone.

The rule reports a direct capture. These shapes are left alone:

- a finalizer that reaches the object only through its parameter;
- a finalizer function literal that reads the object from a package variable;
- a finalizer that reaches the object only through a captured struct or
  another local, as in `h.it` with a closure that reads `h`;
- a stored method value, a method value with a value receiver, and a promoted
  method value;
- a finalizer obtained from a call.

The rule reports the capture at the call and does not analyze a later
assignment to the object variable. The finalizer may read a different value by
the time it runs, but the object is still referenced at registration.

A fix rewrites the function literal to use its parameter in place of the
captured object. The fix is offered only when the finalizer is a function
literal written at the call. A method value has no literal body at the call to
rewrite, and a function literal stored in a variable may be used more than
once, so the fix is withheld. An unnamed or blank parameter takes the object's
name. The fix is also withheld when the object is not an identifier, when the
parameter type differs from the object type, when the finalizer captures
another local, when the object is assigned or its address is taken anywhere,
when the finalizer reassigns its parameter or takes its address, when a local
declaration in the finalizer would shadow the parameter, or when a rewritten
line would exceed the width. A fix is still offered when a rewritten line is
already past the width.

The rule is adapted from staticcheck's SA5005.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/finalizercapture/cmd/finalizercapture
go tool finalizercapture ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -finalizercapture ./...
go tool lesiw.io/tools/cmd/fix -finalizercapture ./...
```
