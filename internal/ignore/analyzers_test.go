package ignore

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type identCheck func(pass *analysis.Pass, ident *ast.Ident)

func identAnalyze(checker identCheck) func(*analysis.Pass) (any, error) {
	return func(pass *analysis.Pass) (any, error) {
		for _, file := range pass.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.FuncDecl:
					checkFuncDecl(node, pass, checker)
				case *ast.GenDecl:
					checkGenDecl(node, pass, checker)
				case *ast.AssignStmt:
					checkAssignStmt(node, pass, checker)
				}
				return true
			})
		}
		return nil, nil
	}
}

func checkFuncDecl(node *ast.FuncDecl, pass *analysis.Pass, checker identCheck) {
	if node.Name != nil && node.Name.Name != "_" {
		checker(pass, node.Name)
	}
}

func checkGenDecl(node *ast.GenDecl, pass *analysis.Pass, checker identCheck) {
	for _, spec := range node.Specs {
		if tspec, ok := spec.(*ast.TypeSpec); ok && tspec.Name.Name != "_" {
			checker(pass, tspec.Name)
		} else if vspec, ok := spec.(*ast.ValueSpec); ok {
			for _, name := range vspec.Names {
				if name.Name != "_" {
					checker(pass, name)
				}
			}
		}
	}
}

func checkAssignStmt(node *ast.AssignStmt, pass *analysis.Pass, checker identCheck) {
	if node.Tok == token.DEFINE {
		for _, expr := range node.Lhs {
			if ident, ok := expr.(*ast.Ident); ok && ident.Name != "_" {
				checker(pass, ident)
			}
		}
	}
}

var (
	publicNames = &analysis.Analyzer{
		Name: "publicnames",
		Doc:  "reports on public names",
		Run: identAnalyze(func(pass *analysis.Pass, ident *ast.Ident) {
			if ident.IsExported() {
				pass.Reportf(ident.Pos(), "%s is public", ident.Name)
			}
		}),
	}
	numberedNames = &analysis.Analyzer{
		Name: "numberednames",
		Doc:  "reports on names with numbers in them",
		Run: identAnalyze(func(pass *analysis.Pass, ident *ast.Ident) {
			for _, r := range ident.Name {
				if r >= '0' && r <= '9' {
					pass.Reportf(ident.Pos(), "%s has numbers", ident.Name)
					break
				}
			}
		}),
	}
	shortNames = &analysis.Analyzer{
		Name: "shortnames",
		Doc:  "reports on single-letter names",
		Run: identAnalyze(func(pass *analysis.Pass, ident *ast.Ident) {
			if len(ident.Name) == 1 {
				pass.Reportf(ident.Pos(), "%s is single letter", ident.Name)
			}
		}),
	}
	underscoreNames = &analysis.Analyzer{
		Name: "underscorenames",
		Doc:  "reports on names containing underscores",
		Run: identAnalyze(func(pass *analysis.Pass, ident *ast.Ident) {
			if strings.Contains(ident.Name, "_") {
				pass.Reportf(ident.Pos(), "%s has underscore", ident.Name)
			}
		}),
	}
	todoAnalyzer = &analysis.Analyzer{
		Name: "todocomments",
		Doc:  "reports on TODO comments",
		Run: func(pass *analysis.Pass) (any, error) {
			for _, file := range pass.Files {
				for _, cg := range file.Comments {
					for _, c := range cg.List {
						if !strings.Contains(c.Text, "TODO") {
							continue
						}
						f := pass.Fset.File(c.Pos())
						pass.Reportf(
							f.LineStart(f.Line(c.Pos())),
							"comment contains TODO",
						)
					}
				}
			}
			return nil, nil
		},
	}
	dotAnalyzer = &analysis.Analyzer{
		Name: "dotcomments",
		Doc:  "reports comments that do not end in a period",
		Run: func(pass *analysis.Pass) (any, error) {
			for _, file := range pass.Files {
				for _, cg := range file.Comments {
					for _, c := range cg.List {
						if !strings.HasSuffix(c.Text, ".") {
							pass.Reportf(c.End()-1,
								"comment does not end in a period",
							)
						}
					}
				}
			}
			return nil, nil
		},
	}
)

// dependentAnalyzer depends on publicNames and uses its results
var dependentAnalyzer = &analysis.Analyzer{
	Name:     "dependent",
	Doc:      "analyzer that depends on publicnames",
	Requires: []*analysis.Analyzer{publicNames},
	Run: func(pass *analysis.Pass) (any, error) {
		// This should have access to publicNames results via pass.ResultOf
		if result, ok := pass.ResultOf[publicNames]; ok {
			// Use the result somehow - report on the first file's package
			// declaration
			_ = result
			if len(pass.Files) > 0 {
				pass.Reportf(pass.Files[0].Package, "dependent analyzer ran")
			}
		} else {
			if len(pass.Files) > 0 {
				pass.Reportf(pass.Files[0].Package, "dependent analyzer missing required result")
			}
		}
		return "dependent result", nil
	},
}

var (
	names = []*analysis.Analyzer{publicNames, numberedNames}
	short = []*analysis.Analyzer{shortNames, underscoreNames}
	all   = []*analysis.Analyzer{
		publicNames, numberedNames, shortNames, underscoreNames,
	}
	todos = []*analysis.Analyzer{todoAnalyzer}
)

// combined runs analyzers as one, since analysistest runs a single analyzer
// per call.
func combined(analyzers ...*analysis.Analyzer) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "combined",
		Doc:  "runs analyzers as one",
		Run: func(pass *analysis.Pass) (any, error) {
			var (
				filtered = Apply(analyzers...)
				results  = make(map[*analysis.Analyzer]any)
				exec     func(*analysis.Analyzer) error
			)
			exec = func(a *analysis.Analyzer) error {
				if _, ok := results[a]; ok {
					return nil
				}
				inputs := make(map[*analysis.Analyzer]any)
				for _, req := range a.Requires {
					if err := exec(req); err != nil {
						return err
					}
					inputs[req] = results[req]
				}
				inner := *pass
				inner.Analyzer = a
				inner.ResultOf = inputs
				result, err := a.Run(&inner)
				if err != nil {
					return err
				}
				results[a] = result
				return nil
			}
			for _, a := range filtered {
				if err := exec(a); err != nil {
					return nil, err
				}
			}
			return nil, nil
		},
	}
}
