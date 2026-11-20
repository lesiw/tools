# lesiw.io/tools/analyzers/assertresult

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/assertresult.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/assertresult)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a case that asserts the value its type
switch already tested.

```go
func report(x any) {
    switch x.(type) { // a case asserts the value again; bind it in the switch
    case int:
        print(x.(int))
    }
}
```

A type switch compares the dynamic type of the value its guard tests against
each case (https://go.dev/ref/spec#Type_switches). A case that lists one type is
entered only when the type matches, so the assertion in it cannot fail. The fix
gives the guard a name for the value:

```go
switch v := x.(type) {
case int:
    print(v)
}
```

The rule reports each switch once. A guard that already names the value is
reported too, as in `switch v := x.(type)` with a case that asserts `x.(int)`,
and the fix reads the name the guard already binds.

Not reported:

- a guard that tests something other than an identifier, as in
  `switch f().(type)`; the rule reads the value through its identifier, so a
  selector or a call is left alone;
- a case that lists several types, or a `default`, since the value is bound at
  the type it already has there and the assertion is still needed;
- an assertion that a write can reach before it, whether in the case or
  through a function literal the case can call, since the assertion then tests
  something the switch did not.

The fix leaves alone an assertion of another value, an assertion of the value
at a type the case does not list, and a comma-ok assertion such as
`v, ok := x.(int)`, which binds two values. A switch whose assertions are all
of those draws no report, since the rule exists for the assertions it can
rewrite.

The rule is adapted from staticcheck's S1034
(https://staticcheck.dev/docs/checks/#S1034), "Use result of type assertion to
simplify cases". Upstream leaves a case alone when it asserts another value at
the case's type; this rule rewrites the assertions of the value the switch
tested, which are the redundant ones, and leaves the others where they are.

## Fixes

The fix reads the value the switch binds in place of each assertion, and binds
it under a new name first when the guard names nothing.

The name is one the switch does not use, and one no switch nested in it binds.
Binding puts the name in scope for every case, so a name a case already reads
would shadow what that read means. The first free name wins: `v` unless the
switch uses it, then `v1`, `v2`, and so on.

Nothing else in the switch changes. A case that writes the value after
asserting it, compares it with nil, or asserts it at another type keeps doing
that to the value the guard tested, since only the assertions the fix replaces
read the new name.

The fix is withheld, and the report kept, when a comment sits inside the text an
edit removes, which is the `x.(type)` in the guard or an assertion the fix
replaces.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/assertresult/cmd/assertresult
go tool assertresult ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -assertresult ./...
go tool lesiw.io/tools/cmd/fix -assertresult ./...
```
