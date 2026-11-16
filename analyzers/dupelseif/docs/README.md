# lesiw.io/tools/analyzers/dupelseif

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dupelseif.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dupelseif)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an else-if whose condition repeats an
earlier condition in the same chain.

```go
func f(n int) int {
    if n > 0 {
        return 1
    } else if n > 0 { // else-if repeats an earlier condition
        return 2
    }
    return 0
}
```

By the if statements section of the spec
(https://go.dev/ref/spec#If_statements), an else-if is an if statement in the
else branch, so it is reached only after every earlier condition was false. A
later condition that matches an earlier one is false too when each condition
reads the same value both times, and no init statement or condition that can
read a different value lies between them. The branch can never run.

An init statement or a condition that can read a different value ends the
comparison with earlier conditions. Conditions after it are still compared with
each other. A call that is neither a conversion nor a pure builtin, a channel
receive, the address of a composite literal, and `len` of a value that can be a
channel can all give a different value the second time. An init can change a
variable an earlier condition read or shadow a name, and an unstable condition
can change one itself. Creating a function literal runs nothing, so effects in
its body do not end the comparison.

Conditions are compared as printed. Parentheses around a whole condition and
line breaks are ignored; parentheses inside it remain, so `(n) > 0` does not
match `n > 0`, and `0 < n` does not match `n > 0`.

An else block ends the chain. A chain inside the block is checked on its own.

Report only: removing the branch would drop any comment inside it or leave a
local declared above it or an import unused. The intended condition is not in
the source.

The rule is inspired by staticcheck's SA4014.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dupelseif/cmd/dupelseif
go tool dupelseif ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dupelseif ./...
```
