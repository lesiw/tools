module lesiw.io/tools

go 1.26.0

tool (
	golang.org/x/tools/cmd/goimports
	lesiw.io/tools/cmd/clerk
	lesiw.io/tools/cmd/vet
)

require (
	github.com/Antonboom/errname v1.1.2
	github.com/google/go-cmp v0.7.0
	golang.org/x/tools v0.48.0
	lesiw.io/checker v0.15.0
	lesiw.io/clerk v0.2.1-0.20260726115654-97532659cfda
	lesiw.io/command v0.0.0-20260726033941-13d34b3faefc
	lesiw.io/errcheck v1.0.0
	lesiw.io/linelen v0.4.0
	lesiw.io/plscheck v0.20.0
	lesiw.io/tidytypes v0.2.0
)

require (
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/telemetry v0.0.0-20260708182218-49f421fb7959 // indirect
	lesiw.io/fs v0.13.0 // indirect
	lesiw.io/prefix v0.1.0 // indirect
	lesiw.io/zeros v0.3.0 // indirect
)
