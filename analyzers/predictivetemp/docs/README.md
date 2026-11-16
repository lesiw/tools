# lesiw.io/tools/analyzers/predictivetemp

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/predictivetemp.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/predictivetemp)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a file created at a fixed path in a shared
directory.

```go
func write() error {
    file, err := os.Create("/tmp/report") // file created at a fixed path in a shared directory; write os.CreateTemp("/tmp", "report")
    if err != nil {
        return err
    }
    return file.Close()
}
```

A fixed name in a shared directory is one file for every process on the
machine, and every user can write there, so another process may already have
created it. The `os.CreateTemp` doc promises that programs calling it
simultaneously "will not choose the same file". A fixed name offers no such
guarantee.

## Checks

A call is reported when the function is `os.Create` or `os.OpenFile` and the
path argument is a string constant in a shared directory. A literal, a named
constant, and a concatenation of constants are all constants. A method that
happens to be called `Create`, and a variable holding `os.Create`, are left
alone.

An `os.OpenFile` is reported only when the call creates the file. `O_CREATE` is
what makes it create anything, and `O_EXCL` is what makes the creation fail
when the name is taken. `O_EXCL` protects the last component alone, so it
settles a name that sits directly in the shared directory:
`os.OpenFile("/tmp/lock", os.O_CREATE|os.O_EXCL, 0o600)` claims the name as a
lock and is not reported. The same flags under a subdirectory are reported,
because another process may have made that subdirectory into a link.

The directories compared are the Unix temporary ones, `/tmp`, `/var/tmp`, and
`/usr/tmp`, and the names macOS gives them, `/private/tmp` and
`/private/var/tmp`. A path anywhere under one of them is reported, so
`/tmp/app/state` counts: a subdirectory with a fixed name is as predictable as
the name in it.

What is not reported:

- A path outside the shared directories, absolute or relative:
  `os.Create("state.json")` and `os.Create("/var/lib/app/state")`.
- The path of a shared directory itself, as `/tmp`.
- A path the analyzer cannot read as a constant: a parameter,
  `os.TempDir() + "/report"`, and `filepath.Join("/tmp", "report")`, whose
  arguments it does not fold. A literal stored in a variable is not followed
  either.
- An `os.OpenFile` whose flag is not an integer constant.
- `os.WriteFile`, which creates a file at a fixed path the same way.

## Fixes

The path is split: the directory becomes the first argument of `os.CreateTemp`,
and the file name becomes its pattern. `os.Create("/tmp/report")` becomes
`os.CreateTemp("/tmp", "report")`. A dot import writes `CreateTemp`, and an
aliased import writes the alias.

`os.CreateTemp` picks the name, so the rewrite opens a different file than the
call did. It is a new file on every run, in mode `0o600` where `os.Create`
passes `0o666` before the umask applies, and nothing removes it:
`os.CreateTemp`'s doc leaves that to the caller. The fix is offered only when
all of these hold:

- No other constant in the package names that file, in either name of its
  directory. A later `os.Remove("/tmp/report")` would remove a file the call
  no longer creates, and a path written as a named constant counts as naming
  it twice, because the constant is still there after the rewrite.
- The call's result is kept. A bare `os.Create(path)` statement drops it, and
  the file the rewrite creates would have a name nothing can read.
- The call lies on one line, and no comment sits inside it.
- The rewrite stays within the width `linelen` measures.

The message names the rewrite where only the layout stands in the way, so a fix
withheld for a line or a comment still says what to write.

An `os.OpenFile` is reported without a fix. `os.CreateTemp` takes a directory
and a pattern and nothing else, so the rewrite would drop the flags and the
mode the call names.

A file name holding a `*` takes no fix either, because `os.CreateTemp`
replaces the last `*` of its pattern with the random string.

A caller that needs the fixed name, as a lock file does, takes the `O_EXCL`
form above, which the rule leaves alone. An `os.OpenFile` that appends to a
fixed name has no form the rule accepts, and a caller that means it leaves an
`//ignore` comment on the call.

The rule is adapted from gosec's `G303`, which matches a path prefix under
`/usr/tmp`, `/var/tmp`, or `/tmp`, covers `os.WriteFile` as well, follows a
local variable to its initializer, and offers no fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/predictivetemp/cmd/predictivetemp
go tool predictivetemp ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -predictivetemp ./...
go tool lesiw.io/tools/cmd/fix -predictivetemp ./...
```
