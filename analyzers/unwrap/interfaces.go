package unwrap

import (
	"go/ast"
	"slices"

	"lesiw.io/tools/internal/header"
	"lesiw.io/tools/internal/signature"
)

// collapseInterfaceTypes pulls an interface type written across lines onto one
// line when the line it lands on fits. go/printer prints an interface type on
// one line only when it holds at most one short field, so a type holding more
// keeps its lines: the printed line would not be the one the line table
// records.
func collapseInterfaceTypes(r *renderer) {
	var (
		tokFile       = r.tokFile
		file          = r.file
		uses, defined = interfaces(file)
		headers       = header.Spans(file)
		sigs          = signature.Spans(file)
		snap          *snapshot
	)
	for _, u := range uses {
		it := u.it
		// The interface a type declaration is written with declares a
		// type rather than typing a value.
		if _, ok := defined[it]; ok {
			continue
		}
		// collapseSignatures and collapseHeaders join a signature or a
		// header whole, so an interface inside one is left to them.
		if signature.In(sigs, it.Pos()) || header.In(headers, it.Pos()) {
			continue
		}
		if sameLine(tokFile, it.Pos(), it.End()) {
			continue
		}
		// A // comment runs to the end of its line, so the lines it
		// spans cannot be joined.
		if lineCommentWithin(file, it.Pos(), it.End()) {
			continue
		}
		// A span the estimate measures as too long is skipped without a
		// render; join measures the printed line anyway.
		if snap == nil {
			snap = r.snapshot()
		}
		if merged, ok := snap.merged(it.Pos(), it.End()); ok && !fits(merged) {
			continue
		}
		joinAligned(r, it.Pos(), it.End(), u.alignment)
	}
}

// interfaceUse is one interface type in a file, with the alignment the lines
// it spans sit in.
type interfaceUse struct {
	it        *ast.InterfaceType
	alignment alignment
}

// interfaces returns every interface type in file, along with the ones a type
// declaration defines.
func interfaces(file *ast.File) (uses []interfaceUse, defined map[*ast.InterfaceType]struct{}) {
	var stack []ast.Node
	defined = make(map[*ast.InterfaceType]struct{})
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *ast.TypeSpec:
			if it, ok := ast.Unparen(n.Type).(*ast.InterfaceType); ok {
				defined[it] = struct{}{}
			}
		case *ast.InterfaceType:
			uses = append(uses, interfaceUse{
				it:        n,
				alignment: alignmentOf(stack),
			})
		}
		return true
	})
	return uses, defined
}

// alignmentOf returns the span of the innermost construct the printer pads to
// its widest entry: a declaration block, a struct type, or a composite
// literal. A join inside one moves the columns of the lines around it.
func alignmentOf(stack []ast.Node) (a alignment) {
	for _, n := range slices.Backward(stack) {
		switch n := n.(type) {
		case *ast.GenDecl:
			if n.Lparen.IsValid() {
				return alignment{lo: n.TokPos, hi: n.End()}
			}
		case *ast.StructType, *ast.CompositeLit:
			return alignment{lo: n.Pos(), hi: n.End()}
		}
	}
	return alignment{}
}
