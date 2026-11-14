# lesiw.io/tools/analyzers/sloppyreassign

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/sloppyreassign.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/sloppyreassign)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an assignment in the init clause of an if,
switch, or for statement whose variable is not used outside the statement.

```go
func load(name string) error {
	var err error
	if err = fetch(name); err != nil { // err can be declared in this if statement
		return err
	}
	return nil
}
```

A short declaration gives `err` the scope where it is read:

```go
func load(name string) error {
	if err := fetch(name); err != nil {
		return err
	}
	return nil
}
```

The rule stands down where:

- any use of the variable outside the statement, read or write;
- a naked return, which reads every named result;
- a use from a function literal inside the statement, or through the variable's
  address, since either can outlive the statement;
- a package-level variable, which another file or a dot import can read;
- a value whose type is not the variable's type, since the declaration would
  give the variable the value's type, and a typed nil stored in an interface
  variable would stop comparing equal to nil;
- a constant value, which the rule does not distinguish: a declaration gives
  a constant its default type, so declaring 1 makes a float64 variable an
  int.

A statement that never reads the variable is left alone: the declaration
there would be unread, and the assignment is a dead store.

An init whose target is not a plain variable is left alone, and so is one that
writes more than one, since the rule follows a single variable.

The rule is adapted from go-critic's sloppyReassign, which reports only an if
whose condition compares the variable with nil and whose body returns it, and
does not ask where else the variable is used.

## When the fix is withheld

The fix has two parts: a short declaration in the header, and the removal of
the variable's local declaration. The declaration stays where the variable is a
named result or the init reads it. A site is reported without a fix where the
declaration cannot go: a spec with a value, one carrying a comment, a short
variable declaration or a range clause, where the name left behind would not
compile, a parameter, or a receiver, which leaves `unusedparams` or
`unusedrecv` to report it. So is a site whose header line the extra byte would
take past `linelen`'s limit.

A named result keeps its name. Dropping it is a signature change, and a result
may be named on purpose.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/sloppyreassign/cmd/sloppyreassign
go tool sloppyreassign ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -sloppyreassign ./...
go tool lesiw.io/tools/cmd/fix -sloppyreassign ./...
```
