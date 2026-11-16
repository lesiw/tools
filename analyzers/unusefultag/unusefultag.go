// Package unusefultag provides an analysis.Analyzer that reports a json
// struct tag on an unexported field or with no name or options.
//
// The encoding/json/v2 documentation states that "Every Go struct corresponds
// to a list of JSON-representable fields" made by leaving out "unexported or
// ignored fields", and that a field's "representation ... can be customized in
// the "json" struct field tag." It also says: "Unexported fields must not have
// any json tags except for json:"-"." A named unexported field has no JSON
// representation. The field is left out under the v1-compatible encoder; a tag
// other than json:"-" on it is an error under strict v2. The tag is reported
// either way. A blank field is unexported. For the unexported-field check, a
// field with several names is read when all of them are unexported; a mixed
// declaration is left to structtag. An empty tag on a mixed declaration is
// still reported.
//
// The first json tag option is the object name. When it is absent the field
// name is used. The documentation says "If the name is not specified, then the
// Go struct field name is used as the JSON object name." A tag with no name
// and no options therefore has no effect. The v1-compatible encoder also gives
// json:"," no effect; that form is a trailing-comma error under strict v2. The
// tag has no name or option, so it is reported.
//
// On an exported field, a tag with no name but with an option, as
// json:",omitempty", is left alone: the field name is used and the option may
// apply. An anonymous struct field is not reported for being unexported: a
// named tag supplies the embedded field's name. A tag with no name and no
// options is reported; a tag with no name but with an option is left alone,
// because options are not checked and embed or a future option may be valid.
// An unexported anonymous field of a non-struct type is reported: it has no
// struct fields to embed, and it is an error under strict v2. json:"-" is left
// alone even on an unexported field; it is allowed there under strict v2.
//
// The x/tools structtag analyzer reports a tag that reflect.StructTag.Get
// cannot read, and reports the unexported field case for json and xml. This
// rule adds the empty json tag and reads no other key.
//
// Packages under encoding/json, including its external test package, are left
// alone, since their tests contain tagged fields that exercise these rules.
//
// The rule is adapted from the x/tools structtag analyzer's unexported-field
// check and is inspired by staticcheck's SA9005, which reports a marshal call
// over a struct with no exported fields. The README catalogs every check and
// exemption.
//
// Report only: removing the tag and exporting the field are different repairs,
// and a tag may be read by another package through reflection, so deleting it
// is not always safe.
package unusefultag

import (
	"go/ast"
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unusefultag",
	Doc:  "reports a json tag on an unexported field or with no name or options",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if skipped(pass.Pkg.Path()) {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if st, ok := n.(*ast.StructType); ok {
				check(pass, st)
			}
			return true
		})
	}
	return nil, nil
}

// skipped reports whether path belongs to encoding/json, whose tests contain
// tagged fields that exercise these rules.
func skipped(path string) bool {
	switch {
	case path == "encoding/json", path == "encoding/json_test":
		return true
	case strings.HasPrefix(path, "encoding/json/"):
		return true
	}
	return false
}

// check reports each field in st with a json tag on an unexported field or
// with no name or options.
func check(pass *analysis.Pass, st *ast.StructType) {
	for _, field := range st.Fields.List {
		if field.Tag == nil {
			continue
		}
		tag, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			continue
		}
		value, ok := reflect.StructTag(tag).Lookup("json")
		if !ok || value == "-" {
			continue
		}
		if names := unexportedNames(field); len(names) > 0 {
			noun := "field"
			if len(names) > 1 {
				noun = "fields"
			}
			pass.Reportf(
				field.Pos(), "json tag on unexported %s %s cannot be used",
				noun, strings.Join(names, ", "),
			)
			continue
		}
		if unexportedEmbedded(pass, field) {
			msg := "json tag on unexported embedded non-struct field cannot be used"
			pass.Reportf(field.Pos(), "%s", msg)
			continue
		}
		checkName(pass, field, value)
	}
}

// unexportedNames returns every name in field when all of them are unexported,
// and nil otherwise.
func unexportedNames(field *ast.Field) (names []string) {
	if len(field.Names) == 0 {
		return nil
	}
	for _, name := range field.Names {
		if name.IsExported() {
			return nil
		}
		names = append(names, name.Name)
	}
	return
}

// unexportedEmbedded reports whether field is an anonymous field of an
// unexported non-struct type, which has no exported fields to include.
func unexportedEmbedded(pass *analysis.Pass, field *ast.Field) bool {
	if len(field.Names) != 0 || exportedEmbedded(field.Type) {
		return false
	}
	typ := embeddedType(pass, field)
	if typ == nil {
		return false
	}
	_, isStruct := typ.Underlying().(*types.Struct)
	return !isStruct
}

// embeddedType returns the type an anonymous field embeds, following aliases
// and pointers, and nil for a named field or an unknown type.
func embeddedType(pass *analysis.Pass, field *ast.Field) types.Type {
	if len(field.Names) != 0 {
		return nil
	}
	typ := pass.TypesInfo.TypeOf(field.Type)
	if typ == nil {
		return nil
	}
	typ = types.Unalias(typ)
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = types.Unalias(pointer.Elem())
	}
	return typ
}

// exportedEmbedded reports whether the name written for an embedded field is
// exported. The field name is the name written in the type expression, even
// when the denoted type has another name.
func exportedEmbedded(expr ast.Expr) bool {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return expr.IsExported()
	case *ast.StarExpr:
		return exportedEmbedded(expr.X)
	case *ast.SelectorExpr:
		return expr.Sel.IsExported()
	case *ast.IndexExpr:
		return exportedEmbedded(expr.X)
	case *ast.IndexListExpr:
		return exportedEmbedded(expr.X)
	}
	return true
}

// checkName reports a tag value with no name and no options.
func checkName(pass *analysis.Pass, field *ast.Field, value string) {
	name, options, _ := strings.Cut(value, ",")
	if name == "" && !hasOption(options) {
		pass.Reportf(field.Pos(), "json tag has no name or options")
	}
}

// hasOption reports whether the comma-separated options list has at least one
// non-empty entry.
func hasOption(options string) bool {
	for option := range strings.SplitSeq(options, ",") {
		if option != "" {
			return true
		}
	}
	return false
}
