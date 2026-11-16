# lesiw.io/tools/analyzers/weakrandom

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/weakrandom.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/weakrandom)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a value from `math/rand` or
`math/rand/v2` that a token, a key, or a password receives.

```go
func newToken() string {
    return fmt.Sprint(rand.Intn(1000000)) // math/rand is not suitable for security-sensitive work; use crypto/rand for a token
}
```

The `math/rand` package documentation says its outputs "might be easily
predictable regardless of how it's seeded" and points to `crypto/rand` "[f]or
random numbers suitable for security-sensitive work". `math/rand/v2` documents
the same limits and documents ChaCha8 as a general-purpose source resistant to
prediction, while the package still points to `crypto/rand` for
security-sensitive work. `crypto/rand` documents a "cryptographically secure
random number generator".

A call to a function or method in either package is reported when its value
reaches a name that says token, key, or password. The name is split at case
changes and non-letters, so `apiKey`, `APIKey`, `tokenID`, `api_key`, and
`tokens` match. A word has to end at a case change or a non-letter, so
`monkey`, `tokenizer`, and `keyboard` do not match, and an all-capitals run
such as `APIKEY` is one word. A value of a named type such as `Token`, `Key`,
or `Password` is also reported.

The paths to a name are:

- an assignment, short declaration, or range value, including a field,
  element, map entry with a constant string key, or dereferenced pointer;
- a variable declaration;
- a return from a function whose name, named result, or type says one of the
  three;
- an argument bound to a parameter whose name or type says one of the three,
  including a variadic parameter, where the function or method is declared in
  the same package;
- a conversion to a named type that says one of the three;
- a field of a struct literal, whether keyed or positional, and the elements
  of a literal whose named type says one of the three;
- a send on a channel that says one of the three;
- an argument that a `Read` fills with random bytes.

`Read` is reported when its argument says one of the three, as in
`rand.Read(key)`. A buffer filled by `Read` carries the value to a later
assignment, as in `rand.Read(buf)` followed by `token := string(buf)`.

Not every expression carries a value. Assignments, operators, conversions, and
`append` carry one to another name. A call carries one through its result only
for a conversion, the builtin `complex`, or a formatting function: the `fmt`
formatting functions, the `strconv` formatting functions, and
`EncodeToString` in `encoding/base32`, `encoding/base64`, and `encoding/hex`.
For example, `len(fmt.Sprint(rand.Intn(10)))` holds a length and
`pick(keys, rand.Intn(len(keys)))` holds a selected key. The argument itself
is still checked when its parameter says one of the three in the same package.

`fmt.Sprint(rand.Intn(10))` assigned to `token` reports, and so does a value
stored in `n` and later assigned to `token`, until another assignment replaces
it. An assignment inside a condition or loop may not run, so it adds to the
value instead of replacing it; an init statement runs once and replaces it. A
value returned from a function in the same package is followed through its
return expressions. A local value does not
carry through an argument into another function, and a function literal
inherits the values its enclosing function carries and is checked where it
appears, including in a package initializer. Package variables are resolved
before the functions, repeated passes follow references from one package
variable to another, and a write to a package variable inside a function
carries to functions checked later, across files in file order.

A composite literal records each field or element at its path, and a read of
that path carries the value. An entry in a map literal whose constant string
key says one of the three is checked at the literal. The whole container does
not carry one, so `token := []int{rand.Intn(10)}` and
`key := map[string]int{"n": rand.Intn(10)}` leave the call alone.

An index selects a value from its base, so a random index does not carry its
source into the result. In `key := keys[rand.Intn(len(keys))]` the selected
key comes from the slice, and the call is left alone. Two non-constant indexes
on the same variable match each other; a constant index matches only itself.

The rule covers every function that returns a random value and every method
in both packages, except the constructors, `Seed`, serialization methods, and
methods on a `ChaCha8` source. `math/rand/v2` documents ChaCha8 as a
general-purpose source resistant to prediction. A `*rand.Rand` built over a
ChaCha8 source in the same expression is left alone; one stored in a variable
or field is reported.

Not reported, deliberately:

- a random value that reaches a name without one of the three words, as
  `count`, `delay`, `secret`, or `nonce`;
- a name where one of the words is part of a longer word, as `monkey`,
  `tokenizer`, or `keyboard`;
- a random value used directly as a map or slice index, or directly to choose
  a branch, as `m[rand.Intn(len(m))]` or `if rand.Intn(2) == 0`; once the value
  is assigned to a token, key, or password, it is reported at the assignment;
- a later result of a function whose name says one of the three, as the error
  in `func newToken() (string, error) { return "", fmt.Errorf(...) }`; the
  first result is paired with the function name, and named results are paired
  by position;
- a later target filled by a function outside this package that returns several
  values, because only local function results are paired with their targets;
- a whole composite literal assigned to a token, key, or password, as
  `token := []int{rand.Intn(10)}`; the fields and elements are checked at the
  paths they fill, so `values[1]` in `values := []int{rand.Intn(10), 0}` is a
  constant and does not carry the random value in `values[0]`;
- a call through a function value, as `next := rand.Intn; next(10)`;
- an argument bound to a parameter of a function or method in another package,
  whose parameter names are not part of its API, as `context.WithValue(ctx,
  fmt.Sprint(rand.Intn(10)), v)`;
- a value passed to a function whose parameter says none of the three, even
  when the body assigns it to a token, because arguments do not carry into
  another function;
- a call in a `_test.go` file, because the file is not part of the built
  package;
- a call to `crypto/rand`, or to a function in another package that happens to
  be named `Intn`;
- a constructor, `Seed`, or a serialization method such as `MarshalBinary`,
  `AppendBinary`, or `UnmarshalBinary`, as `rand.New(rand.NewSource(1))` or
  `rand.NewChaCha8(seed)`; a random method called on the result is still
  reported;
- a method on a `ChaCha8` source, as `source.Uint64()`, or on a `*rand.Rand`
  built over one in the same expression; a `*rand.Rand` stored in a variable
  or field is reported.

The names are a heuristic. A `key` that indexes a map or cache, or a `token`
from a lexer, is reported when it receives a random value even though it is not
security-sensitive. The names do not distinguish those uses. A source is
reported once; when the same value reaches two names, the first kind found is
the message.

Report only: which generator to use depends on the author's requirements, and
`crypto/rand` is the package the Go documentation names.

The rule is adapted from gosec's G404, which reports a fixed list of functions
in the two packages without considering the value's destination.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/weakrandom/cmd/weakrandom
go tool weakrandom ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -weakrandom ./...
```
