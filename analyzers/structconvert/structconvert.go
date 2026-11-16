// Package structconvert provides an analysis.Analyzer that reports a struct
// literal that copies every field of another struct.
//
// The Conversions section of the spec says that, ignoring struct tags, a
// non-constant value x can be converted to type T when x's type and T are not
// type parameters but have identical underlying types
// (https://go.dev/ref/spec#Conversions). That conversion is legal even when
// the tags differ. This rule requires the tags to be identical.
// The literal initializes each field from the field of the same name, so the
// conversion and the literal produce the same value.
//
//	type first struct {
//		Kind string
//	}
//
//	type second struct {
//		Kind string
//	}
//
//	func convert(v first) second {
//		return second{Kind: v.Kind}
//	}
//
// The rule reports a literal that copies every field of one struct into
// another when the two underlying structs are identical. The source and
// destination types must be declared in the package under analysis, possibly
// through an alias, or be unnamed struct types. The README states each
// condition and each exemption.
//
// A fix replaces the literal with a conversion. It is withheld when the
// destination type cannot be written at that position, when a name in the
// rewrite refers to a different object, when a comment would be lost, or when
// the rewritten line would exceed the width. A fix is still offered when a
// replaced line is already past the width.
//
// The rule is adapted from staticcheck's S1016, which ignores tags.
package structconvert

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "structconvert",
	Doc:  "reports a struct literal that copies every field of another struct",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			if lit, ok := n.(*ast.CompositeLit); ok {
				check(pass, file, src, lit, addressed(stack), stack)
			}
			return true
		})
	}
	return nil, nil
}

// addressed returns whether the composite literal at the top of stack is the
// operand of an address-of expression, through any parentheses.
func addressed(stack []ast.Node) bool {
	for i := len(stack) - 2; i >= 0; i-- {
		switch node := stack[i].(type) {
		case *ast.ParenExpr:
			continue
		case *ast.UnaryExpr:
			return node.Op == token.AND
		default:
			return false
		}
	}
	return false
}

// destination returns the source text for a fix, the text for the message,
// and whether the fix may use it.
func destination(pass *analysis.Pass, file *ast.File, src []byte, lit *ast.CompositeLit, stack []ast.Node, name *types.TypeName, dest, litType types.Type) (text, shown string, writable bool) {
	if lit.Type != nil {
		source, ok := sourceText(src, pass.Fset.File(lit.Pos()), lit.Type)
		if ok {
			shown = source
			if anonymous(lit.Type) || multiline(source) {
				if text, ok := typeText(pass, file, dest, lit.Pos()); ok {
					shown = text
				}
			}
			return source, shown, true
		}
	}
	if lit.Type == nil {
		if enclosing, ok := enclosingType(stack, lit); ok {
			source, read := sourceText(
				src, pass.Fset.File(lit.Pos()),
				enclosing,
			)
			if read && sameStruct(pass.TypesInfo.TypeOf(enclosing), dest) {
				shown = source
				if multiline(source) {
					if text, ok := typeText(pass, file, dest, lit.Pos()); ok {
						shown = text
					}
				}
				return source, shown, true
			}
		}
	}
	if text, ok := localName(pass, name, litType, lit.Pos()); ok {
		return text, text, true
	}
	if text, ok := typeText(pass, file, dest, lit.Pos()); ok {
		return text, text, true
	}
	return "", "", false
}

// renderer qualifies package names for one file at one position.
type renderer struct {
	pass  *analysis.Pass
	names map[string]string
	pos   token.Pos
	ok    bool
}

func (r *renderer) qualifier(pkg *types.Package) string {
	if pkg == r.pass.Pkg {
		return ""
	}
	name, good := imports.Qualifier(r.pass, r.names, pkg.Path(), r.pos)
	r.ok = r.ok && good
	return name
}

// typeText returns the text of a type when every name in it resolves at pos.
func typeText(pass *analysis.Pass, file *ast.File, dest types.Type, pos token.Pos) (string, bool) {
	var (
		r = &renderer{
			pass:  pass,
			names: imports.Names(pass, file),
			pos:   pos,
			ok:    true,
		}
		text = types.TypeString(dest, r.qualifier)
	)
	if !r.ok {
		return text, false
	}
	tv, err := types.Eval(pass.Fset, pass.Pkg, pos, text)
	return text, err == nil && tv.IsType() && types.Identical(tv.Type, dest)
}

// genericName returns whether typ is name with type arguments.
func genericName(typ types.Type, name *types.TypeName) bool {
	switch t := typ.(type) {
	case *types.Alias:
		return t.TypeParams().Len() > 0 && t.Obj() == name
	case *types.Named:
		return t.TypeArgs().Len() > 0 && t.Obj() == name
	}
	return false
}

// localName returns the name a rewrite can write for a local type at pos.
func localName(pass *analysis.Pass, name *types.TypeName, original types.Type, pos token.Pos) (string, bool) {
	if name == nil || name.Pkg() != pass.Pkg {
		return "", false
	}
	if genericName(original, name) {
		return "", false
	}
	if imports.Lookup(pass, name.Name(), pos) != name {
		return "", false
	}
	return name.Name(), true
}

func sameStruct(left, right types.Type) bool {
	return left != nil && right != nil &&
		types.Identical(left.Underlying(), right.Underlying())
}

// enclosingType returns the type expression of the element or map entry that
// lit initializes. It searches only the first enclosing composite literal.
func enclosingType(stack []ast.Node, lit *ast.CompositeLit) (ast.Expr, bool) {
	for i := len(stack) - 2; i >= 0; i-- {
		outer, ok := stack[i].(*ast.CompositeLit)
		if !ok {
			continue
		}
		return elementType(outer, lit)
	}
	return nil, false
}

func elementType(outer, lit *ast.CompositeLit) (ast.Expr, bool) {
	switch typ := outer.Type.(type) {
	case *ast.ArrayType:
		for _, elt := range outer.Elts {
			if elt == lit {
				return typ.Elt, true
			}
		}
	case *ast.MapType:
		for _, elt := range outer.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if kv.Key == lit {
				return typ.Key, true
			}
			if kv.Value == lit {
				return typ.Value, true
			}
		}
	}
	return nil, false
}

// check reports lit when it copies every field of one struct into another.
func check(pass *analysis.Pass, file *ast.File, src []byte, lit *ast.CompositeLit, addressOf bool, stack []ast.Node) {
	if addressOf || len(lit.Elts) == 0 {
		return
	}
	var (
		destType                       = pass.TypesInfo.TypeOf(lit)
		dest, destName, destStruct, ok = structType(destType)
	)
	if !ok || destStruct.NumFields() != len(lit.Elts) {
		return
	}
	source, ok := sourceOf(pass, lit, destStruct)
	if !ok {
		return
	}
	srcType := pass.TypesInfo.TypeOf(source)
	srcStructType, _, srcStruct, ok := structType(srcType)
	if !ok {
		return
	}
	if !local(pass, destType) || !local(pass, srcType) {
		return
	}
	srcType = srcStructType
	if types.Identical(dest, srcType) {
		return
	}
	if !types.Identical(destStruct, srcStruct) {
		return
	}
	destText, shown, writable := destination(
		pass, file, src, lit,
		stack, destName, dest, destType,
	)
	value, ok := astexpr.OneLine(source)
	if !ok {
		value, _ = sourceText(src, pass.Fset.File(source.Pos()), source)
	}
	var written string
	if raw, ok := sourceText(src, pass.Fset.File(source.Pos()), source); ok {
		written = destText + "(" + raw + ")"
	} else {
		writable = false
	}
	message := "struct literal copies every field of " + value
	if writable {
		message += "; write " + shown + "(" + value + ")"
	} else {
		message += "; write a conversion of " + value
	}
	diagnostic := analysis.Diagnostic{
		Pos:     lit.Pos(),
		End:     lit.End(),
		Message: message,
	}
	if fixable(pass, src, file, lit, written, writable) {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Write a conversion",
			TextEdits: []analysis.TextEdit{{
				Pos:     lit.Pos(),
				End:     lit.End(),
				NewText: []byte(written),
			}},
		}}
	}
	pass.Report(diagnostic)
}

// sourceOf returns the expression that every selector reads, and whether each
// selector reads that one expression.
func sourceOf(pass *analysis.Pass, lit *ast.CompositeLit, dest *types.Struct) (ast.Expr, bool) {
	values, ok := fields(lit, dest)
	if !ok {
		return nil, false
	}
	return selected(pass, values)
}

// selected returns the expression that every selector reads, and whether each
// field reads that one expression.
func selected(pass *analysis.Pass, values []*ast.SelectorExpr) (ast.Expr, bool) {
	var source ast.Expr
	for _, sel := range values {
		expr := ast.Unparen(sel.X)
		if source == nil {
			source = expr
			continue
		}
		if !astexpr.Same(pass, source, expr) {
			return nil, false
		}
	}
	return source, source != nil
}

// fields returns the selectors that initialize the fields of dest, in
// declaration order. It returns false when the literal does not give one
// selector per field, each naming the field it initializes.
func fields(lit *ast.CompositeLit, dest *types.Struct) ([]*ast.SelectorExpr, bool) {
	if _, keyed := lit.Elts[0].(*ast.KeyValueExpr); keyed {
		return keyedFields(lit, dest)
	}
	return orderedFields(lit, dest)
}

// orderedFields returns the selectors of an unkeyed literal, which must follow
// the field declarations.
func orderedFields(lit *ast.CompositeLit, dest *types.Struct) ([]*ast.SelectorExpr, bool) {
	values := make([]*ast.SelectorExpr, dest.NumFields())
	for i, elt := range lit.Elts {
		sel, ok := selector(elt)
		if !ok || sel.Sel.Name != dest.Field(i).Name() {
			return nil, false
		}
		values[i] = sel
	}
	return values, true
}

// keyedFields returns the selectors of a keyed literal, in field declaration
// order.
func keyedFields(lit *ast.CompositeLit, dest *types.Struct) ([]*ast.SelectorExpr, bool) {
	values := make([]*ast.SelectorExpr, dest.NumFields())
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil, false
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			return nil, false
		}
		sel, ok := selector(kv.Value)
		if !ok || key.Name != sel.Sel.Name {
			return nil, false
		}
		index := fieldIndex(dest, key.Name)
		if index < 0 || values[index] != nil {
			return nil, false
		}
		values[index] = sel
	}
	for _, sel := range values {
		if sel == nil {
			return nil, false
		}
	}
	return values, true
}

func fieldIndex(dest *types.Struct, name string) int {
	for i := range dest.NumFields() {
		if dest.Field(i).Name() == name {
			return i
		}
	}
	return -1
}

// structType returns t with aliases removed, the type name it was declared
// with if any, and its underlying struct.
func structType(t types.Type) (types.Type, *types.TypeName, *types.Struct, bool) {
	if t == nil {
		return nil, nil, nil, false
	}
	var name *types.TypeName
	if alias, ok := t.(*types.Alias); ok {
		name = alias.Obj()
	}
	t = types.Unalias(t)
	if named, ok := t.(*types.Named); ok {
		if name == nil {
			name = named.Obj()
		}
		strct, ok := named.Underlying().(*types.Struct)
		return named, name, strct, ok
	}
	strct, ok := t.Underlying().(*types.Struct)
	return t, name, strct, ok
}

// local returns whether a type and its aliases resolve to the package under
// analysis. An unnamed type has no package to compare.
func local(pass *analysis.Pass, typ types.Type) bool {
	for {
		switch t := typ.(type) {
		case *types.Alias:
			if t.Obj().Pkg() != pass.Pkg {
				return false
			}
			typ = t.Rhs()
		case *types.Named:
			return t.Obj().Pkg() == pass.Pkg
		default:
			return true
		}
	}
}

func selector(e ast.Expr) (*ast.SelectorExpr, bool) {
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	return sel, ok
}

func anonymous(e ast.Expr) bool {
	_, ok := ast.Unparen(e).(*ast.StructType)
	return ok
}

func multiline(text string) bool {
	return strings.ContainsRune(text, '\n')
}

func sourceText(src []byte, file *token.File, n ast.Node) (string, bool) {
	if file == nil {
		return "", false
	}
	lo, hi := file.Offset(n.Pos()), file.Offset(n.End())
	if lo < 0 || hi < lo || hi > len(src) {
		return "", false
	}
	return string(src[lo:hi]), true
}

func fixable(pass *analysis.Pass, src []byte, file *ast.File, lit *ast.CompositeLit, written string, writable bool) bool {
	if !writable {
		return false
	}
	if !fits(src, pass.Fset.File(lit.Pos()), lit, written) {
		return false
	}
	return !commented(file.Comments, lit.Pos(), lit.End())
}

// fits returns whether the rewritten line stays within the width limit. A
// rewrite is allowed when a replaced line is already past the limit.
func fits(src []byte, file *token.File, lit *ast.CompositeLit, written string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	if file == nil {
		return false
	}
	start, end := file.Offset(lit.Pos()), file.Offset(lit.End())
	if start < 0 || end < start || end > len(src) {
		return false
	}
	lineStart := bytes.LastIndexByte(src[:start], '\n') + 1
	lineStop := len(src)
	if next := bytes.IndexByte(src[end:], '\n'); next >= 0 {
		lineStop = end + next
	}
	for line := range bytes.SplitSeq(src[lineStart:lineStop], []byte("\n")) {
		if !linelen.CheckLine(line) {
			return true
		}
	}
	line := make([]byte, 0, start-lineStart+len(written)+lineStop-end)
	line = append(line, src[lineStart:start]...)
	line = append(line, written...)
	line = append(line, src[end:lineStop]...)
	if bytes.ContainsRune(line, '\n') {
		for part := range bytes.SplitSeq(line, []byte("\n")) {
			if !linelen.CheckLine(part) {
				return false
			}
		}
		return true
	}
	return linelen.CheckLine(line)
}

// commented returns whether a comment overlaps the span from pos to end.
func commented(groups []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
