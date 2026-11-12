package tidytypes

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "tidytypes",
	Doc:  "reports redundant type declarations in parameters and results",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				checkFunction(pass, node.Type)
			case *ast.FuncLit:
				checkFunction(pass, node.Type)
			}
			return true
		})
	}
	return nil, nil
}

func checkFunction(pass *analysis.Pass, funcType *ast.FuncType) {
	if funcType == nil {
		return
	}
	if funcType.Params != nil {
		checkFieldList(pass, funcType.Params, "parameter")
	}
	if funcType.Results != nil {
		checkFieldList(pass, funcType.Results, "result")
	}
}

func checkFieldList(pass *analysis.Pass, fields *ast.FieldList, kind string) {
	if len(fields.List) < 2 {
		return
	}
	var redundantGroups [][]*ast.Field
	for i := 0; i < len(fields.List)-1; i++ {
		field := fields.List[i]
		if field.Type == nil {
			continue
		}
		j := i + 1
		for j < len(fields.List) {
			nextField := fields.List[j]
			if nextField.Type == nil {
				break
			}
			if !sameType(pass, field.Type, nextField.Type) {
				break
			}
			j++
		}
		if j > i+1 {
			redundantGroups = append(redundantGroups, fields.List[i:j])
		}
		i = j - 1
	}
	for _, group := range redundantGroups {
		reportRedundantTypes(pass, group, kind, fields.List, redundantGroups)
	}
}

func reportRedundantTypes(pass *analysis.Pass, fields []*ast.Field, kind string, allFields []*ast.Field, allRedundantGroups [][]*ast.Field) {
	if fieldsUnnamed(fields) {
		// Report once on the first field with a fix for the entire group.
		pass.Report(analysis.Diagnostic{
			Pos:     fields[0].Type.Pos(),
			End:     fields[len(fields)-1].Type.End(),
			Message: "redundant type in " + kind + " list",
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: "remove redundant type",
				TextEdits: createUnnamedFieldFix(
					fields, allFields,
					allRedundantGroups,
				),
			}},
		})
		return
	}

	// Report on all but the last field in the group.
	for i := range len(fields) - 1 {
		var (
			field      = fields[i]
			diagnostic = analysis.Diagnostic{
				Pos:     field.Type.Pos(),
				End:     field.Type.End(),
				Message: "redundant type in " + kind + " list",
			}
		)
		if len(field.Names) > 0 {
			// For named fields, remove the redundant type.
			diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
				Message: "remove redundant type",
				TextEdits: []analysis.TextEdit{{
					Pos:     field.Names[len(field.Names)-1].End(),
					End:     field.Type.End(),
					NewText: []byte(""),
				}},
			}}
		}
		pass.Report(diagnostic)
	}
}

func fieldsUnnamed(fields []*ast.Field) bool {
	for _, field := range fields {
		if len(field.Names) > 0 {
			return false
		}
	}
	return true
}

func createUnnamedFieldFix(fields, allFields []*ast.Field, allRedundantGroups [][]*ast.Field) (edits []analysis.TextEdit) {
	for i, field := range fields {
		if i == len(fields)-1 {
			// Last field in redundant group: add "_" and keep type
			edits = append(edits, analysis.TextEdit{
				Pos:     field.Type.Pos(),
				End:     field.Type.Pos(),
				NewText: []byte("_ "),
			})
		} else {
			// Other fields in redundant group: replace type with "_"
			edits = append(edits, analysis.TextEdit{
				Pos:     field.Type.Pos(),
				End:     field.Type.End(),
				NewText: []byte("_"),
			})
		}
	}

	if fieldsUnnamed(allFields) {
	loop:
		// Add "_" to fields not in any group.
		for _, field := range allFields {
			// Check if this field is part of ANY redundant group.
			for _, group := range allRedundantGroups {
				if slices.Contains(group, field) {
					continue loop
				}
			}
			edits = append(edits, analysis.TextEdit{
				Pos:     field.Type.Pos(),
				End:     field.Type.Pos(),
				NewText: []byte("_ "),
			})
		}
	}

	return
}

func sameType(pass *analysis.Pass, t1, t2 ast.Expr) bool {
	// A variadic type never groups: go/types reports ...T as []T,
	// but out, src ...byte is not a legal grouping of
	// out []byte, src ...byte.
	if _, ok := t1.(*ast.Ellipsis); ok {
		return false
	}
	if _, ok := t2.(*ast.Ellipsis); ok {
		return false
	}
	var (
		type1 = pass.TypesInfo.TypeOf(t1)
		type2 = pass.TypesInfo.TypeOf(t2)
	)
	if type1 == nil || type2 == nil {
		// Types were not loaded. Fall back to string comparisons.
		return types.ExprString(t1) == types.ExprString(t2)
	}
	return types.Identical(type1, type2)
}
