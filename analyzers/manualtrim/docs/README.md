# lesiw.io/tools/analyzers/manualtrim

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/manualtrim.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/manualtrim)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if statement that trims a string prefix
by reslicing it.

```go
func f(s, prefix string) string {
    if strings.HasPrefix(s, prefix) { // manual prefix trim; use strings.TrimPrefix
        s = s[len(prefix):]
    }
    return s
}
```

The `strings.TrimPrefix` doc says "If `s` doesn't start with `prefix`, `s` is
returned unchanged" (https://pkg.go.dev/strings#TrimPrefix). An if statement
that tests the prefix and then reslices the string by its length is equivalent
to an unconditional call:

```go
s = strings.TrimPrefix(s, prefix)
```

The condition and the body must use the same value and the same prefix. The
body must hold one statement: an assignment of the resliced value back to the
value the condition tested, with `=`, one target, and one value. A call is not
compared, since two calls can return different values. The right side must be
`s[len(prefix):]`, with the builtin `len` and no high bound; a high bound, as
in `s[len(prefix):len(s)]`, is left alone.

The target must be a name, or a field, index, or dereference rooted at a name.
An index target whose operand can be a map is left alone, because the key
would be inserted on the path where the prefix is absent. A field or
dereference below a map index is reported, since the condition already
dereferences the map value.

An if statement is left alone in these cases:

- it has an init clause, because the init and its declarations are lost in the
  replacement;
- it has an else branch, because the else branch must stay conditional.

An else-if gets a block in the replacement, since a bare assignment cannot
follow `else`. The replacement contains the package name the file uses, or the
unqualified `TrimPrefix` for a dot import; when that name is shadowed, the
message reports the shadowing and offers no fix. The fix is withheld when a
comment sits in the text it would replace, and the diagnostic is still
reported.

The rule is inspired by staticcheck's S1017.

Fixable: the if statement becomes `s = strings.TrimPrefix(s, prefix)`, with
the package name the file uses, or the unqualified `TrimPrefix` for a dot
import. A chained if gets a block.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/manualtrim/cmd/manualtrim
go tool manualtrim ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -manualtrim ./...
go tool lesiw.io/tools/cmd/fix -manualtrim ./...
```
