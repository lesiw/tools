// Package emptylines provides an analysis.Analyzer that reports blank lines
// that separate nothing inside a block.
//
// A blank line at either edge of a block has no statement on the far side of
// it, and a single blank line already separates the content above it from the
// content below, so a run of two holds one line too many. gofmt keeps the line
// at an edge and collapses a run to one, so a run survives only in a file
// nothing has formatted.
//
// The README states which blank lines the rule leaves alone, and what each fix
// writes.
//
// The rule is adapted from revive's empty-lines, which reports a blank line at
// each edge of a block and nothing between them.
package emptylines

import (
	"bytes"
	"go/ast"
	"go/token"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "emptylines",
	Doc:  "reports blank lines that separate nothing inside a block",
	Run:  run,
}

type checker struct {
	pass    *analysis.Pass
	tokFile *token.File
	blank   []bool // indexed by line, so index 0 is unused
	blocks  []block
}

// blankRun is a stretch of blank lines, counting lines from one.
type blankRun struct {
	first, last int
}

type block struct {
	open, close         token.Pos
	openLine, closeLine int
	stmtFirst, stmtLast int // zero when the block holds no statement
}

func run(pass *analysis.Pass) (any, error) {
	for _, syntax := range pass.Files {
		tokFile := pass.Fset.File(syntax.FileStart)
		src, err := pass.ReadFile(tokFile.Name())
		if err != nil {
			return nil, err
		}
		// A file that changed size since it was parsed leaves the line table
		// the fixes index and the bytes they read disagreeing.
		if tokFile.Size() != len(src) {
			continue
		}
		c := &checker{
			pass:    pass,
			tokFile: tokFile,
			blank:   blankLines(tokFile, syntax, src),
			blocks:  blocks(tokFile, syntax),
		}
		c.check()
	}
	return nil, nil
}

func (c *checker) check() {
	for _, r := range blankRuns(c.blank) {
		b, ok := c.enclosing(r)
		if ok {
			c.reportRun(r, b)
		}
	}
}

// reportRun reports the run r of block b. A run at an edge of the block loses
// every line it holds; a run inside the block keeps one.
func (c *checker) reportRun(r blankRun, b block) {
	switch {
	case atStart(r, b):
		c.edge(b.open, r, "at the start of a block")
	case atEnd(r, b):
		c.edge(b.close, r, "at the end of a block")
	case r.last > r.first:
		// The run keeps its first line. The report lands on the line above
		// it, since the line the edit starts at holds nothing to point at.
		c.deleteLines(
			c.tokFile.LineStart(r.first-1), r.first+1, r.last,
			"more than one blank line in a row",
			"Remove the extra blank "+plural(r.last-r.first),
		)
	}
}

// edge reports a run at an edge of a block, which loses every line it holds.
func (c *checker) edge(pos token.Pos, r blankRun, where string) {
	lines := plural(r.last - r.first + 1)
	c.deleteLines(pos, r.first, r.last,
		"blank "+lines+" "+where,
		"Remove the blank "+lines,
	)
}

// plural returns the word for n blank lines.
func plural(n int) string {
	if n == 1 {
		return "line"
	}
	return "lines"
}

// deleteLines reports the lines from first to last at pos, and offers the edit
// that deletes them.
func (c *checker) deleteLines(pos token.Pos, first, last int, msg, fix string) {
	c.pass.Report(analysis.Diagnostic{
		Pos:     pos,
		Message: msg,
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: fix,
			TextEdits: []analysis.TextEdit{{
				Pos: c.tokFile.LineStart(first),
				End: c.tokFile.LineStart(last + 1),
			}},
		}},
	})
}

// enclosing returns the innermost block whose interior holds the run r.
func (c *checker) enclosing(r blankRun) (block, bool) {
	for _, b := range slices.Backward(c.blocks) {
		if b.openLine < r.first && r.last < b.closeLine {
			return b, true
		}
	}
	return block{}, false
}

// atStart reports whether r is the blank line under the opening brace of b. A
// statement on the brace's line comes before the run, which leaves the run
// between two statements. A comment on that line is not a statement, so a run
// below it is still the start of the block.
func atStart(r blankRun, b block) bool {
	return r.first == b.openLine+1 && b.stmtFirst != b.openLine
}

// atEnd reports whether r is the blank run over the closing brace of b. A
// statement on the brace's line comes after the run, so the run is a break
// between two statements.
func atEnd(r blankRun, b block) bool {
	return r.last == b.closeLine-1 && b.stmtLast != b.closeLine
}

// lineAt returns the line pos is written on, ignoring //line directives. The
// line table is indexed by the lines the file is written on, and a directive
// can name a line past its end.
func lineAt(tokFile *token.File, pos token.Pos) int {
	return tokFile.PositionFor(pos, false).Line
}

// blankLines returns the lines of src that hold nothing, counting from one. A
// line inside a comment or a literal holds that token's text, so an empty line
// of a block comment is not blank.
func blankLines(tokFile *token.File, syntax *ast.File, src []byte) []bool {
	var (
		lines = bytes.Split(src, []byte{'\n'})
		blank = make([]bool, len(lines)+1)
	)
	for i, text := range lines {
		blank[i+1] = source.Blank(text)
	}
	// A file that ends with a newline leaves an empty element for the line
	// after it, and no run starts there.
	if bytes.HasSuffix(src, []byte{'\n'}) {
		blank[len(lines)] = false
	}
	for _, c := range syntax.Comments {
		cover(blank, tokFile, c.Pos(), c.End())
	}
	ast.Inspect(syntax, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok {
			cover(blank, tokFile, lit.Pos(), lit.End())
		}
		return true
	})
	return blank
}

// cover marks the lines between first and last as holding text. A single-line
// token covers nothing, since its own line holds its delimiters.
func cover(blank []bool, tokFile *token.File, first, last token.Pos) {
	end := lineAt(tokFile, last)
	for l := lineAt(tokFile, first) + 1; l < end; l++ {
		blank[l] = false
	}
}

// blankRuns returns every stretch of blank lines in the file.
func blankRuns(blank []bool) (out []blankRun) {
	for l := 1; l < len(blank); l++ {
		if !blank[l] {
			continue
		}
		r := blankRun{first: l}
		for l+1 < len(blank) && blank[l+1] {
			l++
		}
		r.last = l
		out = append(out, r)
	}
	return out
}

// blocks returns the blocks of the file in source order, so the last block
// that holds a run is the innermost.
func blocks(tokFile *token.File, syntax *ast.File) (out []block) {
	ast.Inspect(syntax, func(n ast.Node) bool {
		stmt, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		b := block{
			open:      stmt.Lbrace,
			close:     stmt.Rbrace,
			openLine:  lineAt(tokFile, stmt.Lbrace),
			closeLine: lineAt(tokFile, stmt.Rbrace),
		}
		if len(stmt.List) > 0 {
			b.stmtFirst = lineAt(tokFile, stmt.List[0].Pos())
			b.stmtLast = lineAt(tokFile, stmt.List[len(stmt.List)-1].End())
		}
		out = append(out, b)
		return true
	})
	return out
}
