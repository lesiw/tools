# lesiw.io/tools/analyzers/wrapperarg

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/wrapperarg.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/wrapperarg)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call that passes an argument a
convenience wrapper already supplies.

```go
func splitEvery(s, sep string) []string {
    return strings.SplitN(s, sep, -1) // strings.SplitN with a negative count can be replaced by strings.Split
}
```

`strings.Split`'s doc says "It is equivalent to SplitN with a count of -1", so
the count adds a third argument and nothing else.

## Checks

The entries and their docs:

- `wg.Add(-1)` for `wg.Done()`: "It is equivalent to Add(-1)."
- `buf.Truncate(0)` for `buf.Reset()`: "Reset is the same as
  Buffer.Truncate(0)."
- `draw.DrawMask(dst, r, src, sp, nil, mp, op)` for
  `draw.Draw(dst, r, src, sp, op)`: "Draw calls DrawMask with a nil mask", and
  DrawMask's doc says "A nil mask is treated as opaque", so `mp` does not
  affect the result.
- `strings.SplitN(s, sep, -1)`, `bytes.SplitN(b, sep, -1)`,
  `strings.SplitAfterN(s, sep, -1)`, and `bytes.SplitAfterN(b, sep, -1)` for
  `Split` and `SplitAfter`: "It is equivalent to SplitN with a count of -1."
  The `SplitAfter` doc says the same of `SplitAfterN`. A count below zero is
  the same case, so `-2` is reported too.
- `strings.Map` and `bytes.Map` with `unicode.ToUpper`, `unicode.ToLower`, or
  `unicode.ToTitle` for the matching `ToUpper`, `ToLower`, or `ToTitle`.
  `strings.ToUpper` maps every letter to upper case, and `unicode.ToUpper`
  maps one rune to upper case, so the two forms produce the same result. The
  lower and title pairs are the same.
- `bytes.Replace(b, old, new, -1)` for `bytes.ReplaceAll(b, old, new)`.
  Replace's doc gives a negative count "no limit on the number of
  replacements", and ReplaceAll replaces every non-overlapping instance.
  `strings.Replace` takes the same shape, and `strreplaceall` reports it
  with a count of `-1`.

The rule matches the fixed argument as it is written at the call: a negative
integer literal for a count, the literal `-1` for a delta, an integer literal
with the value zero, `nil`, or one of the three `unicode` case mappings. A
count held in a constant or a variable, and a mapping held in a variable, are
left alone.

Every argument the fix deletes has to be evaluable without running code or
panicking, since deleting it removes that work. `DrawMask`'s mask point is the
argument this matters for: in `draw.DrawMask(dst, r, src, sp, nil, next(), op)`
the rewrite would remove the call to `next()`, so that call is reported without
a fix.

The call has to name the function through a selector. A call through a dot
import is left alone, because a name in scope could shadow the target. A
call through a function value is left alone, since there is no selector to
rename. A method expression passes the receiver as its first argument, so the
table's indexes do not apply. A call on an interface is left alone, since the
implementation is not visible here. The diagnostic uses the file's name for a
package, so an import named `str` reads `str.SplitN` and `str.Split`, and it
names a method with its declaring type.

A call is left alone when the receiver's type embeds the wrapper's type and
declares a method with the target's name: `c.Add(-1)` on a type that declares
`Done` would reach that method after the rewrite.

A call inside the target's declaration is left alone, since the rewrite
would call the function from its body. That covers the package that
declares the wrapper: `sync.WaitGroup.Done` is `wg.Add(-1)`, and rewriting it
to `wg.Done()` would recurse.

go-critic's `wrapperFunc` also names `http.HandlerFunc(http.NotFound)`, which
this rule does not report: the conversion's value has type `http.HandlerFunc`,
and `http.NotFoundHandler()` returns `http.Handler`, so the replacement would
not compile where the `HandlerFunc` type is needed.

## Fixes

The fix renames the function at the selector and deletes the fixed arguments,
so the call keeps the receiver expression it already has.

The fix is withheld in three cases, and the call is reported in each:

- a comment sits in the text it would delete, as in
  `strings.SplitN(s, sep /* kept */, -1)`;
- the edit would leave a local variable with no read outside the arguments the
  rule deletes, since the declaration would not compile. A variable whose only
  use is an assignment counts, and so does `mp := image.Point{}` followed by
  `DrawMask(..., nil, mp, op)`;
- a deleted argument names something from a dot-imported package, since the
  import could be left unused, as in a `strings.Map` call with `ToTitle`
  under `. "unicode"`.

The rule is adapted from go-critic's `wrapperFunc`. The `wrapperfunc` analyzer
in this suite uses that name for a rule about declared forwarding functions.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/wrapperarg/cmd/wrapperarg
go tool wrapperarg ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -wrapperarg ./...
go tool lesiw.io/tools/cmd/fix -wrapperarg ./...
```
