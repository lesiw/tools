# lesiw.io/tools/analyzers/afterhttperror

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/afterhttperror.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/afterhttperror)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an http.Error call without a return after it.

```go
func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed) // http.Error does not stop the handler
		w.WriteHeader(http.StatusOK)
	}
}
```

http.Error writes the response and returns. The handler continues and writes a
second response. The client gets a status line followed by whatever the rest of
the handler wrote.

```go
if r.Method != http.MethodGet {
	http.Error(w, "method", http.StatusMethodNotAllowed)
	return
}
```

The idea comes from go-critic's returnAfterHttpError.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/afterhttperror/cmd/afterhttperror
go tool afterhttperror ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -afterhttperror ./...
```
