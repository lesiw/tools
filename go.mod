module lesiw.io/tools

go 1.26.0

tool (
	golang.org/x/tools/cmd/goimports
	lesiw.io/tools/cmd/clerk
	lesiw.io/tools/cmd/fix
	lesiw.io/tools/cmd/reflow
	lesiw.io/tools/cmd/vet
)

require (
	github.com/Antonboom/errname v1.1.2
	github.com/google/go-cmp v0.7.0
	golang.org/x/tools v0.52.0
	lesiw.io/clerk v0.3.0
	lesiw.io/errcheck v1.0.0
)

require (
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20261009151841-97e8ea77aa2a // indirect
)
