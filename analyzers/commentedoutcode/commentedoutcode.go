// Package commentedoutcode provides an analysis.Analyzer that reports a block
// of commented-out code.
//
// The spec says a general comment "acts like a space", that any other "acts
// like a newline", and that comments "serve as program documentation"
// (https://go.dev/ref/spec#Comments). A statement left in a comment is not
// type-checked, not formatted, and not run by a test, so it can disagree with
// the code beside it and nothing reports the disagreement. Version control
// keeps the statement.
//
// A comment on one line is treated as a note. A longer comment inside a
// function body is read as statements, and outside one as declarations.
//
// The README catalogs the checks and the exemptions.
//
// The diagnostic carries no fix. Deleting the comment is the remedy, and a
// comment can hold a note on one of its lines ("// save(item) // helps the
// test") that would be left with nothing to refer to.
//
// The rule is inspired by go-critic's commentedOutCode, which reports a
// comment inside a function body.
package commentedoutcode

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "commentedoutcode",
	Doc:  "reports a block of commented-out code",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		tokFile := pass.Fset.File(file.FileStart)
		// A read failure ends the pass rather than leaving the file's
		// comments unclassified, since the report would then depend on the
		// driver's bytes rather than on the file.
		src, err := pass.ReadFile(tokFile.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", tokFile.Name(), err)
		}
		docs := documented(file)
		for _, group := range file.Comments {
			if docs[group] || directive(group) {
				continue
			}
			if annotates(tokFile, src, group) || !code(file, group) {
				continue
			}
			pass.Report(analysis.Diagnostic{
				Pos:     group.Pos(),
				End:     group.End(),
				Message: "commented-out code",
			})
		}
	}
	return nil, nil
}

// annotates reports whether code precedes the comment group on its line. Such
// a comment annotates that line, and a statement left behind takes lines of
// its own.
func annotates(tokFile *token.File, src []byte, group *ast.CommentGroup) bool {
	// A driver that parses other bytes than ReadFile returns puts the
	// position outside the line, so the bounds are checked rather than
	// trusted.
	start := tokFile.Offset(group.Pos())
	if start > len(src) {
		return true // the bytes do not cover the position; leave it alone
	}
	return !source.Blank(src[source.Start(src, start):start])
}

// directive reports whether one of the group's comments is a directive, which
// a tool reads and a reader does not.
func directive(group *ast.CommentGroup) bool {
	for _, comment := range group.List {
		text, ok := strings.CutPrefix(comment.Text, "//")
		if ok && isDirective(text) {
			return true
		}
	}
	return false
}

// isDirective reports whether text, with its slashes removed, is a comment
// directive. The colon rule is go/ast's isDirective, which is not exported;
// the words are the ones a tool reads with no colon.
func isDirective(text string) bool {
	fields := strings.Fields(text)
	if len(fields) > 0 {
		switch fields[0] {
		case "nolint", "sys", "sysnb":
			return true
		}
	}
	for _, prefix := range []string{"line ", "extern ", "export "} {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	colon := strings.IndexByte(text, ':')
	if colon <= 0 || colon+1 >= len(text) {
		return false
	}
	for i := 0; i <= colon+1; i++ {
		if i == colon {
			continue
		}
		if b := text[i]; !('a' <= b && b <= 'z' || '0' <= b && b <= '9') {
			return false
		}
	}
	return true
}

// code reports whether the comment group holds code.
func code(file *ast.File, group *ast.CommentGroup) bool {
	comment := group.Text()
	// A note fits on one line, so a comment on one line is left alone. The
	// group's text ends with a newline.
	if strings.Count(comment, "\n") < 2 {
		return false
	}
	if insideBody(file, group.Pos()) {
		stmts, ok := parseBody(comment)
		return ok && block(stmts)
	}
	decls, ok := parseDecls(comment)
	return ok && slices.ContainsFunc(decls, declaration)
}

// insideBody reports whether pos is inside a function body, including the body
// of a function literal.
func insideBody(file *ast.File, pos token.Pos) bool {
	var inside bool
	ast.Inspect(file, func(n ast.Node) bool {
		if inside {
			return false
		}
		var body *ast.BlockStmt
		switch n := n.(type) {
		case *ast.FuncDecl:
			body = n.Body
		case *ast.FuncLit:
			body = n.Body
		}
		if body != nil && body.Pos() <= pos && pos < body.End() {
			inside = true
			return false
		}
		return true
	})
	return inside
}

// parseBody returns the statements text holds when it parses as a function
// body.
func parseBody(text string) ([]ast.Stmt, bool) {
	src := "package p\n\nfunc _() {\n" + text + "\n}\n"
	file, err := parser.ParseFile(
		token.NewFileSet(), "", src,
		parser.SkipObjectResolution,
	)
	if err != nil || len(file.Decls) != 1 {
		return nil, false
	}
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || len(fn.Body.List) == 0 {
		return nil, false
	}
	return fn.Body.List, true
}

// parseDecls returns the declarations text holds when it parses as the top
// level of a file.
func parseDecls(text string) ([]ast.Decl, bool) {
	file, err := parser.ParseFile(
		token.NewFileSet(), "", "package p\n\n"+text+"\n",
		parser.SkipObjectResolution,
	)
	if err != nil {
		return nil, false
	}
	return file.Decls, true
}

// block reports whether stmts hold code rather than a note.
func block(stmts []ast.Stmt) bool {
	if _, ok := stmts[0].(*ast.LabeledStmt); ok {
		return false // a colon makes the first line a heading
	}
	return slices.ContainsFunc(stmts, statement)
}

// statement reports whether a statement is one a comment leaves behind: a
// call on its own, a declaration, or a statement that holds another statement.
func statement(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.ExprStmt:
		_, ok := ast.Unparen(stmt.X).(*ast.CallExpr)
		return ok
	case *ast.GoStmt, *ast.DeferStmt:
		return true // the statement holds a call and nothing else
	case *ast.DeclStmt:
		return declaration(stmt.Decl)
	}
	return holds(stmt)
}

// holds reports whether a compound statement holds statements of its own.
func holds(stmt ast.Stmt) bool {
	var body *ast.BlockStmt
	switch stmt := stmt.(type) {
	case *ast.IfStmt:
		body = stmt.Body
	case *ast.ForStmt:
		body = stmt.Body
	case *ast.RangeStmt:
		body = stmt.Body
	case *ast.SwitchStmt:
		body = stmt.Body
	case *ast.TypeSwitchStmt:
		body = stmt.Body
	case *ast.SelectStmt:
		body = stmt.Body
	case *ast.BlockStmt:
		body = stmt
	}
	return body != nil && len(body.List) > 0
}

// declaration reports whether a declaration is one a comment leaves behind. A
// type declaration states a shape ("type count int"), and a function
// declaration without a body is a signature.
func declaration(decl ast.Decl) bool {
	switch decl := decl.(type) {
	case *ast.FuncDecl:
		return decl.Body != nil
	case *ast.GenDecl:
		return decl.Tok != token.TYPE
	}
	return false
}

// documented returns the comment groups that a reader sees as documentation: a
// package, a top-level declaration, and a field. go doc does not show the doc
// comment of a declaration inside a body, and inside a body such a group reads
// as code. A spec's trailing comment needs no entry here: code annotation is
// skipped before the group is read.
func documented(file *ast.File) map[*ast.CommentGroup]bool {
	var (
		docs = make(map[*ast.CommentGroup]bool)
		add  = func(group *ast.CommentGroup) {
			if group != nil {
				docs[group] = true
			}
		}
	)
	add(file.Doc)
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			add(decl.Doc)
		case *ast.GenDecl:
			add(decl.Doc)
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					add(spec.Doc)
				case *ast.ValueSpec:
					add(spec.Doc)
				case *ast.ImportSpec:
					add(spec.Doc)
				}
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if field, ok := n.(*ast.Field); ok {
			add(field.Doc)
		}
		return true
	})
	return docs
}
