// Package typename names a type the way a reader writes it.
package typename

import (
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Of returns a qualifier that names a type from another package by that
// package's name, so a message reads http.Header rather than net/http.Header.
func Of(pass *analysis.Pass) types.Qualifier {
	return func(p *types.Package) string {
		if p == pass.Pkg {
			return ""
		}
		return p.Name()
	}
}
