// Package buildtag holds what the go command reads from a file's build
// constraints, for the analyzers that reason about them: the tags a file name
// gives, the header a // +build line is read from, and the GOOS and GOARCH
// values the tags name.
//
// The GOOS and GOARCH lists are go/build's tables (internal/syslist, Go 1.27),
// which keep a target the toolchain no longer builds because a file name
// still names it.
package buildtag

import (
	"bytes"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// HeaderEnd returns the position just past the header the go command reads a
// build line from, and false for a file whose source it cannot read. The
// package doc comment and anything after the header are not constraints.
func HeaderEnd(pass *analysis.Pass, file *ast.File) (token.Pos, bool) {
	tokFile := pass.Fset.File(file.FileStart)
	if tokFile == nil {
		return token.NoPos, false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil || tokFile.Size() != len(src) {
		return token.NoPos, false
	}
	return tokFile.Pos(endOfHeader(src)), true
}

// endOfHeader returns the offset just after the last blank line in the run of
// blank lines and // comments at the start of src. A /* */ comment or the
// package clause ends the run.
func endOfHeader(src []byte) (end int) {
	var i int
	if bytes.HasPrefix(src, []byte("\ufeff")) {
		i = len("\ufeff")
	}
	for i < len(src) {
		next := bytes.IndexByte(src[i:], '\n')
		if next < 0 {
			next = len(src)
		} else {
			next += i + 1
		}
		line := bytes.TrimSpace(src[i:next])
		switch {
		case len(line) == 0:
			end = next
		case !bytes.HasPrefix(line, []byte("//")):
			return end
		}
		i = next
	}
	return end
}

// NameTags returns the tags a file name gives, and nil for a name that gives
// none. This copies go/build's rule for a name that matches *_GOOS, *_GOARCH
// or *_GOOS_GOARCH: the part after the first underscore decides, from its last
// one or two elements, with a trailing test dropped.
func NameTags(name string) []string {
	name, _, _ = strings.Cut(filepath.Base(name), ".")
	i := strings.Index(name, "_")
	if i < 0 {
		return nil
	}
	parts := strings.Split(name[i:], "_")
	if n := len(parts); n > 0 && parts[n-1] == "test" {
		parts = parts[:n-1]
	}
	switch n := len(parts); {
	case n >= 2 && KnownOS[parts[n-2]] && KnownArch[parts[n-1]]:
		return []string{parts[n-2], parts[n-1]}
	case n >= 1 && (KnownOS[parts[n-1]] || KnownArch[parts[n-1]]):
		return []string{parts[n-1]}
	}
	return nil
}

// KnownOS holds the GOOS values the go command matches a file name against.
var KnownOS = map[string]bool{
	"aix":       true,
	"android":   true,
	"darwin":    true,
	"dragonfly": true,
	"freebsd":   true,
	"hurd":      true,
	"illumos":   true,
	"ios":       true,
	"js":        true,
	"linux":     true,
	"nacl":      true,
	"netbsd":    true,
	"openbsd":   true,
	"plan9":     true,
	"solaris":   true,
	"wasip1":    true,
	"windows":   true,
	"zos":       true,
}

// UnixOS holds the GOOS values the unix build tag matches. It is not a list of
// names a file is tagged with.
var UnixOS = map[string]bool{
	"aix":       true,
	"android":   true,
	"darwin":    true,
	"dragonfly": true,
	"freebsd":   true,
	"hurd":      true,
	"illumos":   true,
	"ios":       true,
	"linux":     true,
	"netbsd":    true,
	"openbsd":   true,
	"solaris":   true,
}

// KnownArch holds the GOARCH values the go command matches a file name
// against.
var KnownArch = map[string]bool{
	"386":         true,
	"amd64":       true,
	"amd64p32":    true,
	"arm":         true,
	"armbe":       true,
	"arm64":       true,
	"arm64be":     true,
	"loong64":     true,
	"mips":        true,
	"mipsle":      true,
	"mips64":      true,
	"mips64le":    true,
	"mips64p32":   true,
	"mips64p32le": true,
	"ppc":         true,
	"ppc64":       true,
	"ppc64le":     true,
	"riscv":       true,
	"riscv64":     true,
	"s390":        true,
	"s390x":       true,
	"sparc":       true,
	"sparc64":     true,
	"wasm":        true,
}
