# lesiw.io/tools/analyzers/typeassertchain

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/typeassertchain.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/typeassertchain)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a chain of type assertions on one value.

```go
func f(x any) {
	if v, ok := x.(int); ok { // if-else chain of type assertions; use a type switch
		print(v)
	} else if v, ok := x.(string); ok {
		print(v)
	}
}
```

A link is a header, `if v, ok := x.(T); ok`, followed by its body. A chain is
the whole if-else chain: two or more links that assert the same expression,
ending with no else or with a plain else block.

A chain writes the header once per type. A type switch writes it once, and
lists the types as cases.

The idea comes from go-critic's typeAssertChain.

Fixable: the chain becomes a type switch, one case per link, and the switch
binds the name the links give the value: a link that binds the blank identifier
contributes no name. A chain whose links all bind the blank identifier writes
`switch x.(type)`, since a type switch rejects a blank on the left of `:=`.

Not reported:

- links that assert different expressions, since a switch states the expression
  once;
- links whose assertion reads the same but names different objects, since a
  link's binding is in scope for the next link;
- a link whose condition is not the ok result;
- a chain that asserts one type twice, since the spec requires the case types
  to "all be different"; the second spelling may be an alias, which a type
  switch reads as the same type;
- a chain whose head is itself an `else if` link, since a switch cannot follow
  else;
- a chain whose links bind two different names, since one switch binds one name
  for every case;
- a branch that breaks out of a statement around the chain, since the switch
  would hold the break and the branch's target would move;
- a body or a final else that names an ok result, which a type switch does not
  bind;
- a blank-binding link whose body names the guard name, which the switch would
  bind in that clause;
- a final else that names a link's value, since every link has failed by then
  and its binding is the zero value of its type, where a default clause binds
  the name to the value and type of the asserted expression;
- an expression that contains a channel receive or a call that is not a
  conversion.

The fix is withheld when:

- a body or a final else declares the guard name, which a case clause and a
  default clause declare in the same block, so the declaration would collide;
- a header holds a comment the rewrite would replace;
- a header has no line of its own: the chain's `if` or the closing brace does
  not begin its line;
- the links give no name and every body asserts the value to its own case's
  type, which `typeswitchvar` reports in the switch this fix would write;
- the chain is the only statement of an else block, since `elseif` removes those
  braces while this fix rewrites the header, and `else switch` does not parse.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/typeassertchain/cmd/typeassertchain
go tool typeassertchain ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -typeassertchain ./...
go tool lesiw.io/tools/cmd/fix -typeassertchain ./...
```
