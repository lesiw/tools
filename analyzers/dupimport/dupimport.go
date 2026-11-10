// Package dupimport provides an analysis.Analyzer that reports one package
// imported twice under different names.
//
// Two names for one package make every use site ask which name was meant, and
// the compiler accepts it. Like go-critic's dupImport.
package dupimport

import (
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "dupimport",
	Doc:  "reports one package imported twice under different names",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		seen := make(map[string]struct{})
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || path == "C" {
				continue
			}
			// A blank import of a path the file already imports is the
			// go:linkname idiom, and it is deliberate. A dot import
			// brings the package's names into scope, which the named
			// import does not, so the two serve different purposes.
			if spec.Name != nil {
				if spec.Name.Name == "_" || spec.Name.Name == "." {
					continue
				}
			}
			if _, ok := seen[path]; ok {
				pass.Reportf(spec.Pos(), "%s is imported more than once", path)
				continue
			}
			seen[path] = struct{}{}
		}
	}
	return nil, nil
}
