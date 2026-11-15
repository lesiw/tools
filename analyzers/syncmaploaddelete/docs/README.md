# lesiw.io/tools/analyzers/syncmaploaddelete

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/syncmaploaddelete.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/syncmaploaddelete)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `sync.Map` load followed by a delete of
the same key.

```go
func forget(m *sync.Map, key string) {
    if v, ok := m.Load(key); ok { // load and delete can be replaced by sync.Map.LoadAndDelete
        m.Delete(key)
        use(v)
    }
}
```

`sync.Map`'s documentation says `LoadAndDelete` "deletes the value for a key,
returning the previous value if any", and that its loaded result "reports
whether the key was present", which is what `Load` reports through `ok`.
The pair leaves a window between the load and the delete, where a value another
goroutine stores is deleted unseen. `LoadAndDelete` returns the value it
deletes, so nothing goes unseen.

The condition has to be the flag itself: with `ok && done`, a key the load found
stays in the map when `done` is false, and `LoadAndDelete` would remove it.

The receiver and the key have to be written the same way in both calls, with
identifiers and fields compared by the object they name, and indexes, literals,
and dereferences compared one by one. A call is left out, because two calls may
differ, so `m.Load(next())` followed by `m.Delete(next())` is left alone.

The calls have to be `sync.Map`'s own `Load` and `Delete`. A `sync.Map` held by
value and a method promoted through an embedded one both count; a local type
with methods of those names does not.

A load written above its if has to sit directly above it, and that if may not
have an init, because the init would run after the key was gone.

A delete that does not open the block is left alone. The delete moves to where
the load is written, so the statements above the delete would run with the key
gone.

A file whose Go version is below 1.15 is left alone, because `LoadAndDelete` is
not in `sync.Map` at that version.

The rule is adapted from go-critic's `syncMapLoadAndDelete`, which reports the
same pair when it is written above the if, and offers no fix.

Fixable: the load becomes a call to `LoadAndDelete`, and the delete goes.

A body that holds only the delete leaves the if with nothing to do, so the if
goes. When the load is in the header and declares its names, they go with it:
`if _, ok := m.Load(key); ok { m.Delete(key) }` becomes `m.LoadAndDelete(key)`.
When the header assigns to names declared outside and something after the if
still reads the flag, the assignment stays and the condition goes: `if v, ok =
m.Load(key); ok { m.Delete(key) }` becomes `v, ok = m.LoadAndDelete(key)`.

A load written above its if keeps its assignment whenever something after the
if reads the flag. When nothing after the if reads it, one call stands for both
statements. When only the value is read, the flag is written as `_`, and where
the flag was the only name the assignment declared, `:=` becomes `=`.

The report offers no fix in these cases:

- an if with an else whose body holds only the delete, because the else would go
  with it;
- a label on the if of a load written above it, because a goto can land there
  without running the load;
- a label on the delete, because it would be left naming the next statement;
- a comment the edit would delete or leave describing a different statement,
  wherever it sits between the load and the if;
- a name the edit would leave declared and unread, or written with no
  declaration left;
- a line the edit writes that would run past the width `linelen` is set to, with
  no width set meaning nothing overflows;
- a receiver or a key with no one-line form, as a raw string has none, where the
  fix writes the call on one line;
- an else-if whose body holds only the delete, because a call cannot stand where
  the else requires an if.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/syncmaploaddelete/cmd/syncmaploaddelete
go tool syncmaploaddelete ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -syncmaploaddelete ./...
go tool lesiw.io/tools/cmd/fix -syncmaploaddelete ./...
```
