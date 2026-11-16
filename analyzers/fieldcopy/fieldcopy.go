// Package fieldcopy provides an analysis.Analyzer that reports a struct
// literal that copies every field of a same-shaped struct whose tags differ.
//
// The Conversions section of the spec ignores struct tags when it compares two
// struct types, so a value of one type converts to the other even when their
// tags differ (https://go.dev/ref/spec#Conversions). The conversion produces
// the destination type, tags included, so it and the literal have the same
// value.
//
//	type first struct {
//		Kind string `json:"kind"`
//	}
//
//	type second struct {
//		Kind string `json:"type"`
//	}
//
//	func copy(v first) second {
//		return second{Kind: v.Kind}
//	}
//
// The rule reports the literal when the tags differ. structconvert reports the
// identical-tag case.
//
// A fix replaces the literal with a conversion. It is withheld when the
// destination type cannot be written at that position, when a comment would be
// lost, or when the rewritten line would exceed the width. A fix is still
// offered when a replaced line is already past the width.
//
// The rule is adapted from staticcheck's S1016, which ignores tags.
package fieldcopy

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
	Name: "fieldcopy",
	Doc:  "reports a struct literal that copies every field of a same-shaped struct whose tags differ",
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
				check(pass, file, src, lit, addressed(stack))
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

// check reports lit when it copies every field of a struct whose tags differ.
func check(pass *analysis.Pass, file *ast.File, src []byte, lit *ast.CompositeLit, addressOf bool) {
	match, ok := candidate(pass, lit, addressOf)
	if !ok {
		return
	}
	value, ok := astexpr.OneLine(match.source)
	sourceFile := pass.Fset.File(match.source.Pos())
	if !ok {
		value, _ = sourceText(src, sourceFile, match.source)
	}
	raw, rawOK := sourceText(src, sourceFile, match.source)
	destText, destOK := typeText(pass, file, match.destType, lit.Pos())
	destOK = destOK && resolved(pass, match.destType, destText, lit.Pos())
	writable := rawOK && destOK
	var written string
	if writable {
		written = destText + "(" + raw + ")"
	}
	message := "struct literal copies every field of " + value
	if writable {
		message += "; write " + destText + "(" + value + ")"
	} else {
		message += "; write a conversion of " + value
	}
	diagnostic := analysis.Diagnostic{
		Pos:     lit.Pos(),
		End:     lit.End(),
		Message: message,
	}
	if fixable(src, file, pass.Fset.File(lit.Pos()), lit, written, writable) {
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

// match is a literal and the destination type and source expression of its
// field copy.
type match struct {
	destType types.Type
	source   ast.Expr
}

// candidate returns the destination type and source expression of a literal
// whose fields are a copy between structs that differ only in tags.
func candidate(pass *analysis.Pass, lit *ast.CompositeLit, addressOf bool) (match, bool) {
	if addressOf || len(lit.Elts) == 0 {
		return match{}, false
	}
	var (
		destType = pass.TypesInfo.TypeOf(lit)
		dest, ok = structType(destType)
	)
	if !ok || dest.NumFields() != len(lit.Elts) {
		return match{}, false
	}
	source, ok := sourceOf(pass, lit, dest)
	if !ok {
		return match{}, false
	}
	srcType := pass.TypesInfo.TypeOf(source)
	src, ok := structType(srcType)
	if !ok {
		return match{}, false
	}
	if !local(pass, destType) || !local(pass, srcType) {
		return match{}, false
	}
	if !types.IdenticalIgnoreTags(dest, src) {
		return match{}, false
	}
	if types.Identical(dest, src) {
		return match{}, false
	}
	if !types.ConvertibleTo(srcType, destType) {
		return match{}, false
	}
	return match{destType: destType, source: source}, true
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

// resolved returns whether the bare name in text still names typ at pos. A
// local declaration can hide the type that types.Eval finds in the package
// scope.
func resolved(pass *analysis.Pass, typ types.Type, text string, pos token.Pos) bool {
	var obj *types.TypeName
	switch t := typ.(type) {
	case *types.Alias:
		obj = t.Obj()
	case *types.Named:
		obj = t.Obj()
	default:
		return true
	}
	name := obj.Name()
	if text != name && !strings.HasPrefix(text, name+"[") {
		return true
	}
	return imports.Lookup(pass, name, pos) == obj
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

// structType returns the underlying struct of t, with aliases removed.
func structType(t types.Type) (*types.Struct, bool) {
	if t == nil {
		return nil, false
	}
	t = types.Unalias(t)
	if named, ok := t.(*types.Named); ok {
		t = named.Underlying()
	}
	strct, ok := t.Underlying().(*types.Struct)
	return strct, ok
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

func selector(e ast.Expr) (*ast.SelectorExpr, bool) {
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	return sel, ok
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

func fixable(src []byte, file *ast.File, tokFile *token.File, lit *ast.CompositeLit, written string, writable bool) bool {
	if !writable {
		return false
	}
	if !fits(src, tokFile, lit, written) {
		return false
	}
	return !commented(file.Comments, lit.Pos(), lit.End())
}

// fits returns whether the rewritten line stays within the width limit. A
// rewrite is allowed when a replaced line is already past the limit.
func fits(src []byte, tokFile *token.File, lit *ast.CompositeLit, written string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	start, end := tokFile.Offset(lit.Pos()), tokFile.Offset(lit.End())
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
