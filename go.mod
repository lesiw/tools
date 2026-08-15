module lesiw.io/tools

go 1.25.0

tool (
	golang.org/x/tools/cmd/goimports
	lesiw.io/tools/cmd/clerk
	lesiw.io/tools/cmd/fmt
	lesiw.io/tools/cmd/vet
)

require (
	github.com/Antonboom/errname v1.1.2
	golang.org/x/tools v0.48.0
	lesiw.io/boolset v0.1.0
	lesiw.io/checker v0.16.0
	lesiw.io/clerk v0.3.0
	lesiw.io/ctxguard v0.1.0
	lesiw.io/ctxname v0.1.0
	lesiw.io/errcheck v1.0.0
	lesiw.io/errfmt v0.1.0
	lesiw.io/linelen v0.6.0
	lesiw.io/linewrap v0.1.0
	lesiw.io/singlefield v0.1.0
	lesiw.io/strictvar v0.1.0
	lesiw.io/testcmp v0.1.0
	lesiw.io/testhelpers v0.1.0
	lesiw.io/tidytypes v0.2.0
	lesiw.io/timeafter v0.1.0
)

require (
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/telemetry v0.0.0-20260708182218-49f421fb7959 // indirect
)
