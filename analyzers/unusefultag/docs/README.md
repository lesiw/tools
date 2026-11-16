# lesiw.io/tools/analyzers/unusefultag

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unusefultag.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unusefultag)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a json struct tag on an unexported field
or with no name or options.

```go
type row struct {
    name string `json:"name"` // json tag on unexported field name cannot be used
}
```

The encoding/json/v2 documentation states "Every Go struct corresponds to a
list of JSON-representable fields" and that the list leaves out "unexported or
ignored fields". That documentation also says "Unexported fields must not have
any `json` tags except for `json:"-"`." A named unexported field has no JSON
representation. The field is left out under the v1-compatible encoder; a tag
other than `json:"-"` on it is an error under strict v2. The tag is reported
either way. A blank field is unexported as well. A field with several names is
reported when all of them are unexported; a declaration that mixes exported and
unexported names is left to `structtag`, which reports the unexported name. An
empty tag on a mixed declaration is still reported.

The first option in a json tag is the object name. When it is absent, the field
name is used. The documentation says "If the name is not specified, then the Go
struct field name is used as the JSON object name." A tag with no name and no
options therefore has no effect. The v1-compatible encoder also gives
`json:","` no effect; that form is a trailing-comma error under strict v2. The
tag has no name or option, so it is reported.

On an exported field, a tag with no name but with an option,
`json:",omitempty"`, is left alone: the field name is used, and the option may
apply. Options are not checked, so an unknown option, or one that does nothing
for the field's type, is left alone.

An anonymous struct field is not reported for being unexported:
`json:"inner"` supplies the embedded field's name. A tag with no name and no
options is reported; a tag with no name but with an option is left alone, even
though strict v2 may reject it, because options are not checked and `embed` or a
future option may be valid. An unexported anonymous field of a non-struct type
is reported: it has no struct fields to embed, and it is an error under strict
v2. `json:"-"` is left alone
even on an unexported field; it is allowed there under strict v2.
On an exported field, the member is `-` for `json:"-,"` under the
v1-compatible encoder; the v1 `encoding/json` documentation says a field named
`"-"` can be generated with the tag `"-,"`. Its trailing comma is an error under
strict v2.

The suite's `structtag` analyzer also reports an unexported named field with a
non-empty json tag other than `json:"-"`, so such a field may get two
diagnostics when both analyzers run. A non-empty xml tag is reported by
`structtag` alone, and an empty json tag here alone. `structtag` also reports a
tag `reflect.StructTag.Get` cannot read; this rule does not.

Packages under `encoding/json`, including its external test package, are left
alone, since their tests contain tagged fields that exercise these rules.

The rule is adapted from the x/tools `structtag` analyzer's unexported-field
check and is inspired by staticcheck's SA9005, which reports a marshal call over
a struct with no exported fields.

Report only: removing the tag and exporting the field are different repairs,
and a tag may be read by another package through reflection, so deleting it is
not always safe.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unusefultag/cmd/unusefultag
go tool unusefultag ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unusefultag ./...
```
