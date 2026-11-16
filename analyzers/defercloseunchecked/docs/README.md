# lesiw.io/tools/analyzers/defercloseunchecked

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/defercloseunchecked.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/defercloseunchecked)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a deferred call to `Close` on a writer, or
to `Flush`, whose error is discarded.

```go
func write(w *bufio.Writer, p []byte) error {
    if _, err := w.Write(p); err != nil {
        return err
    }
    defer w.Flush() // deferred Flush discards its error
    return nil
}
```

A `Close` can return an error, and a `Write` can return before its data
reaches the underlying writer, while the data is still buffered.
`bufio.Writer.Flush` "writes any buffered data to the underlying io.Writer",
and `gzip.Writer.Close` flushes "any unwritten data to the underlying
io.Writer" and writes the GZIP footer. The Go spec's Defer statements section
says "If the deferred function has any return values, they are discarded when
the function completes", so the error is lost. The same section evaluates the
function value and parameters when the defer statement executes. A method
value saves its receiver: "The expression x is evaluated and saved during the
evaluation of the method value; the saved copy is then used as the receiver in
any calls." So the defer statement captures the receiver. A write after the
defer statement counts until the variable is assigned another file, because
the deferred `Close` still uses the saved receiver.

A `Close` is reported on an `*os.File` opened for writing, or on a writer with
a `Flush` method that returns an error. A `Close` on an `*os.File` is reported
when the file came from `os.Create` or `os.CreateTemp`, from an `os.OpenFile`
call with `O_WRONLY` or `O_RDWR`, from `os.Stdout` or `os.Stderr`, or when its
origin is unknown and the function writes through it. `os.Create` opens the
file for reading and writing, and `os.OpenFile` sets an access mode only from
`O_RDONLY`, `O_WRONLY`, or `O_RDWR`. A file from `os.Open` or `os.Pipe` is
left alone. A `Close` on a `net.Conn` or `io.PipeWriter` is left alone because
its static type has no `Flush` method. A `Flush` is reported when it returns an
error.
Test files are not checked; a close error there is a test failure at most.

A file is followed through local identifiers, conversions, type assertions,
dereferences, address-of expressions, and a one-element literal. A file whose
origin cannot be determined is left alone when its static type is `os.File` or
holds one, as with a file from a helper or a package variable, unless the
function writes through the same identifier. An interface value is
judged by its origin and the methods its static type has. A file whose last
assignment before the defer is on a conditional path is left alone, unless the
function writes through it. A write in a branch that ends in `break` or `goto`
is left alone. A write is tracked on the identifier it names, so a write
through one alias does not mark another. A write in a deferred call
registered before the `Close` is not counted, since that call runs after the
`Close`. A deferred call registered after the `Close` runs first and is
counted.

Not reported:

- a `Close` or `Flush` call inside a deferred function literal, as in
  `defer func() { f.Close() }()`, because only the call the defer names is
  examined; the suite's `errcheck` reports the literal's calls, while the
  standalone command does not check them; a defer statement inside a literal
  is checked on its own;
- a call that does not name a method, such as the builtin `defer close(ch)`;
- a `Close` or `Flush` whose signature returns no error, such as
  `http.Flusher.Flush`;
- a `Close` with an unknown origin on a writer with no `Flush` method, such as
  a `net.Conn` or an `io.PipeWriter`;
- a `Close` on a file from `os.Open`, or on a reader with an unknown origin
  whose static type has no `Write` method, as in `defer body.Close()` on an
  `io.ReadCloser`;
- a method value held in a variable and deferred later, such as
  `defer closeFile()` after `closeFile := f.Close`;
- any other method name.

`errcheck` skips a deferred call, because its error is often handled on a
best-effort basis. This rule reports a `Close` or `Flush` that discards an
error.

The rule is adapted from gosec's G307, which reports a deferred method that
returns an error. This rule covers `Close` and `Flush`, and requires a `Close`
to have a writer as its receiver. gosec no longer reports it; G307 now names an
`os.Create` permission check.

Report only. Reading the error means replacing the defer with a call near the
end of the function, or with a function literal that reads the result; the
repair differs with the surrounding code.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/defercloseunchecked/cmd/defercloseunchecked
go tool defercloseunchecked ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -defercloseunchecked ./...
```
