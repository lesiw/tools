# lesiw.io/tools/analyzers/weakrsa

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/weakrsa.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/weakrsa)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to `rsa.GenerateKey` with a constant
key size below 2048 bits.

```go
func generate() (*rsa.PrivateKey, error) {
    return rsa.GenerateKey(rand.Reader, 1024) // RSA key size 1024 is below the recommended 2048 bits
}
```

The `crypto/rsa` package documentation has a "Minimum key size" section:
`rsa.GenerateKey` returns an error for a key of less than 1024 bits, and such
keys "are insecure and should not be used". The `rsa1024min=0` GODEBUG setting
suppresses that error. NIST SP 800-57 Part 1 Rev. 5 recommends at least 2048
through 2030 (https://doi.org/10.6028/NIST.SP.800-57pt1r5). By default, Go
accepts a 1024-bit key, and the rule reports it for that recommendation.

The rule checks the second argument of a direct call to `rsa.GenerateKey`. A
call through a function value is left alone. The rule is limited to
`rsa.GenerateKey`; `rsa.GenerateMultiPrimeKey` takes its bit count as the third
argument.

A constant key size below 2048 is reported, including 0 and negative values. A
literal, a named constant, a conversion of a constant, and an expression such
as `1024/2` are all constant. A bare variable, a conversion of a variable, and
a call with a non-constant result are left alone, because they are not constant
expressions. A constant expression that uses a local variable, as `len(buf)`
does, is reported. The argument may be parenthesized, and a call through a dot
import or an alias reaches the same function. A call in a `_test.go` file is
left alone, where a test uses a small key.

The fix raises the key size to 2048 bits by replacing the argument with the
decimal `2048`. It is withheld for a constant below 1024, because the call
returns an error and the fix would remove that error path. It is withheld when
a comment overlaps the argument, because the replacement would remove it. It is
withheld when the argument uses a named constant, because the declaration would
still be 1024. It is withheld when the argument uses a local variable, a name a
dot import brings in, or a package-qualified name, because the replacement
could leave the local variable or the import unused; the fix is withheld even
when the import has another use.

A deliberate 1024-bit key is kept with an `//ignore:weakrsa` directive on the
line with the key size.

The rule is adapted from gosec's `G403`, which reports a key below 2048 bits.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/weakrsa/cmd/weakrsa
go tool weakrsa ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -weakrsa ./...
go tool lesiw.io/tools/cmd/fix -weakrsa ./...
```
