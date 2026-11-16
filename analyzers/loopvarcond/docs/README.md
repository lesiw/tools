# lesiw.io/tools/analyzers/loopvarcond

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/loopvarcond.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/loopvarcond)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `for` loop whose condition tests a
variable the loop never changes.

```go
func f(i, n int) {
    for i < n { // loop condition never changes
        use(i)
    }
}
```

The For statements section of the Go spec says that a `for` statement with a
condition evaluates it "before each iteration" and runs the body while the
condition is true (https://go.dev/ref/spec#For_statements). A condition built
from variables that the body and post statement leave alone evaluates to the same
value at every test, so it cannot become false on a later iteration.

## Checks

A loop is reported when its condition is made of identifiers, constants,
`nil`, literals, parentheses, conversions, qualified constants, and the unary
operators `!`, `-`, `+`, and `^`, joined by binary operators. The condition has
to name at least one variable, every variable in it has to be a local variable,
and neither the body nor a reachable post statement may write one.

The common fault is a post statement that increments a variable other than the
one the condition tests:

```go
func f(n, j int) {
    for i := 0; i < n; j++ {
        use(i)
    }
}
```

A write to a variable the condition does not test does not suppress the report.
The condition below still names `i` and `n`, and `other` does not affect either:

```go
func f(n, other int) {
    for i := 0; i < n; {
        other++
        use(i)
    }
}
```

A variable declared again in the body is a different variable, so a shadowed
loop counter does not count as a write to the outer one.

## Exemptions

A condition with a call, a field, an index, a dereference, or a receive is left
alone, since its value can change without a write to a variable in the
condition. `for i < len(xs)` and `for p.field < n` are examples. A qualified
constant such as `time.Minute` is not a field. A type assertion, a slice
expression, or a composite literal is left alone as well.

The Conversions section of the Go spec gives a conversion the value of its
operand in the new type (https://go.dev/ref/spec#Conversions). A conversion of
a basic, pointer, channel, or function value names the same value as its
operand. A conversion of a slice is left alone; a slice-to-slice conversion
copies only the header. A conversion of an array or struct is left alone, since
it can copy mutable contents. A conversion of a map, interface, or type
parameter is left alone as well. `string(buf)` can change when an element of
`buf` changes, for example.

A package-level variable is left alone, including one from a dot import.
Another goroutine can assign it without a write in the loop.

A variable of struct or array type is left alone. A field or element write
changes it without an assignment to the variable, and the rule does not follow
those writes. The Assignments section gives the field or element the stored
value (https://go.dev/ref/spec#Assignment_statements). A type parameter is left
alone because the rule does not follow its type set.

A local variable whose address is taken before the loop ends is left alone,
since a write through the pointer can change it during the loop:

```go
func f(i, n int) {
    p := &i
    for i < n {
        *p++
    }
}
```

A method value with a pointer receiver selected from an addressable variable
that is not itself a pointer is left alone before the loop ends. The Method
values section says the value takes the variable's address
(https://go.dev/ref/spec#Method_values):

```go
func f(c counter, limit counter) {
    bump := c.bump
    for c < limit {
        bump()
    }
}
```

A call to a pointer-receiver method takes the same address
(https://go.dev/ref/spec#Calls), and the method can keep it after the call
returns. A variable already of pointer type supplies the
pointer value, so its method value does not change the variable. A method value
with a value receiver copies the variable, so it does not change the variable.

A local variable captured by a function literal before the loop ends is left
alone when a write can reach it while the loop runs, even when the literal only
reads it. The Function literals section says the variable is shared between the
surrounding function and the literal, and survives as long as it is accessible
(https://go.dev/ref/spec#Function_literals). The surrounding function can write
it while the literal runs in another goroutine. A write after the loop, or no
write at all, leaves the report in place.

```go
func f() {
    done := false
    go func() { done = true }()
    for !done {
        use(done)
    }
}
```

The loop body and post statement count as writes when they assign, increment,
decrement, or assign through a range clause. A method call with a value receiver
cannot change the variable, so it does not count.

## Report only

The tool cannot tell whether the author meant to change the tested variable or
to leave the loop through a `break`, a `return`, or another path in the body,
so no fix is offered. A loop that leaves through a direct `break`, `return`, or
`panic` is still reported, since its condition remains the same. A post
statement is not counted when the body's direct statements, blocks, or complete
if/else branches always leave that way; an exit inside a switch, select, nested
loop, or goto is not followed. An unlabelled continue in a nested loop does not
reach this loop's post statement; a labeled one can.

A condition that a constant init statement makes false before the first test is
left out when the rule can fold it, since the loop cannot run; `badcond`
reports the simple comparison form. A condition that is false for another
reason before the first test is still reported, since the condition would
evaluate to the same value if the body ran. A condition that panics before the
first test is reported as well, since the analyzer does not execute the
condition.

The rule is adapted from staticcheck's SA4008, which applies only to a loop
with an init, condition, and post statement and reads the variable the init
assigns. This rule applies to the condition itself, so it also covers a loop
without an init or post statement.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/loopvarcond/cmd/loopvarcond
go tool loopvarcond ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -loopvarcond ./...
```
