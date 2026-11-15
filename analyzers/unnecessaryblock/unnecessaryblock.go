// Package unnecessaryblock provides an analysis.Analyzer that reports a
// redundant block statement.
//
// The declarations and scope section of the spec says the scope of a name
// declared inside a function "ends at the end of the innermost containing
// block". A block that declares nothing scopes nothing, so the fix drops its
// braces and lets its statements join the block around them.
//
// A block that declares a name keeps its braces, since the braces may be where
// the name's scope should end. A case or comm clause is already a scope, so a
// block that is the whole of one is reported even when it declares a name. The
// README lists the blocks that keep their braces.
//
// The rule is adapted from go-critic's unnecessaryBlock.
package unnecessaryblock

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "unnecessaryblock",
	Doc:  "reports a redundant block statement",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		c := checker{pass: pass, src: src}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BlockStmt:
				c.check(n)
			case *ast.CaseClause:
				c.checkClause(n, n.Body)
			case *ast.CommClause:
				c.checkClause(n, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

type checker struct {
	pass *analysis.Pass
	src  []byte
}

// check reports the blocks in a statement list that add nothing.
func (c checker) check(block *ast.BlockStmt) {
	for _, stmt := range block.List {
		inner, ok := blockOf(stmt)
		if !ok {
			continue
		}
		if len(c.pass.TypesInfo.Scopes[inner].Names()) > 0 {
			continue
		}
		if jumpedOver(block, inner) {
			continue
		}
		c.report(inner)
	}
}

// checkClause reports the blocks in a clause body that add nothing. The clause
// scopes its statements already, so a block that is the whole body is
// redundant even when it declares a name.
func (c checker) checkClause(clause ast.Node, body []ast.Stmt) {
	for _, stmt := range body {
		block, ok := blockOf(stmt)
		if !ok {
			continue
		}
		if len(body) == 1 {
			if c.redeclares(clause, block) {
				continue
			}
		} else if len(c.pass.TypesInfo.Scopes[block].Names()) > 0 {
			continue
		}
		if jumpedOver(clause, block) {
			continue
		}
		c.report(block)
	}
}

// redeclares reports whether moving the block's names into the clause's
// existing scope would declare one of them again.
func (c checker) redeclares(clause ast.Node, block *ast.BlockStmt) bool {
	scope := c.pass.TypesInfo.Scopes[clause]
	for _, name := range c.pass.TypesInfo.Scopes[block].Names() {
		if scope.Lookup(name) != nil {
			return true
		}
	}
	return false
}

// blockOf returns the block under any labels in front of stmt, and false for
// any other statement.
func blockOf(stmt ast.Stmt) (*ast.BlockStmt, bool) {
	for {
		labeled, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			break
		}
		stmt = labeled.Stmt
	}
	block, ok := stmt.(*ast.BlockStmt)
	return block, ok
}

// jumpedOver reports whether a goto in owner jumps from before block to a
// label after it. The scope test cannot see every declaration the jump
// crosses: a blank var declares no name, and a generator can introduce one the
// AST does not show. Go rejects a jump over either.
func jumpedOver(owner ast.Node, block *ast.BlockStmt) bool {
	var (
		gotos  []*ast.BranchStmt
		labels = make(map[string]token.Pos)
	)
	ast.Inspect(owner, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			// A label belongs to the function that declares it.
			return false
		case *ast.LabeledStmt:
			labels[n.Label.Name] = n.Label.Pos()
		case *ast.BranchStmt:
			if n.Tok == token.GOTO {
				gotos = append(gotos, n)
			}
		}
		return true
	})
	for _, jump := range gotos {
		target, ok := labels[jump.Label.Name]
		if ok && jump.Pos() < block.Pos() && target > block.End() {
			return true
		}
	}
	return false
}

func (c checker) report(block *ast.BlockStmt) {
	c.pass.Report(analysis.Diagnostic{
		Pos:     block.Pos(),
		End:     block.End(),
		Message: "redundant block statement",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message:   "Remove the braces",
			TextEdits: c.edits(block),
		}},
	})
}

// edits returns the edits that remove the braces of block.
func (c checker) edits(block *ast.BlockStmt) []analysis.TextEdit {
	if edit, ok := c.emptyBlock(block); ok {
		return []analysis.TextEdit{edit}
	}
	return []analysis.TextEdit{c.brace(block.Lbrace), c.brace(block.Rbrace)}
}

// emptyBlock returns the edit that removes a block holding nothing but white
// space, so that its line does not survive as a blank one. A block that holds
// a comment, or shares its line with anything else, does not qualify.
func (c checker) emptyBlock(block *ast.BlockStmt) (analysis.TextEdit, bool) {
	var (
		file   = c.pass.Fset.File(block.Pos())
		open   = file.Offset(block.Lbrace)
		close  = file.Offset(block.Rbrace)
		first  = source.Start(c.src, open)
		last   = source.End(c.src, close)
		before = source.Blank(c.src[first:open])
		after  = source.Blank(c.src[close+1 : last])
		inside = source.Blank(c.src[open+1 : close])
	)
	if !(before && after && inside) {
		return analysis.TextEdit{}, false
	}
	return analysis.TextEdit{Pos: file.Pos(first), End: file.Pos(last)}, true
}

// brace returns the edit that removes the brace at pos, and its whole line
// when the brace is alone on it.
func (c checker) brace(pos token.Pos) analysis.TextEdit {
	var (
		file = c.pass.Fset.File(pos)
		off  = file.Offset(pos)
	)
	if !source.Alone(c.src, off, off+1) {
		return analysis.TextEdit{Pos: pos, End: pos + 1}
	}
	return analysis.TextEdit{
		Pos: file.Pos(source.Start(c.src, off)),
		End: file.Pos(source.End(c.src, off)),
	}
}
