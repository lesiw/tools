// Package blankimports provides an analysis.Analyzer that reports a blank
// import in a non-main, non-test file with no comment naming its path.
//
// The imports section of the spec says, "To import a package solely for its
// side-effects (initialization), use the blank identifier as explicit package
// name." A blank import triggers the imported package's initialization, and
// no declaration from that package appears in the file through it.
//
// A comment on the import or on the declaration explains the import when it
// contains the import path.
//
// A file whose package clause is main and a test file are left alone. A blank
// import of unsafe is left alone whether or not a //go:linkname directive is
// present. A blank import of embed is left alone in a file with a //go:embed
// directive.
//
// Report only: the repair is a comment or a removal, and only the author
// knows which the import is for.
//
// The rule is adapted from revive's blank-imports.
package blankimports

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "blankimports",
	Doc:  "reports a blank import in a non-main, non-test file with no comment naming its path",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		checkFile(pass, file)
	}
	return nil, nil
}

// checkFile reports the blank imports in file with no comment for them.
func checkFile(pass *analysis.Pass, file *ast.File) {
	name := pass.Fset.Position(file.FileStart).Filename
	if file.Name.Name == "main" || strings.HasSuffix(name, "_test.go") {
		return
	}
	embedded := hasDirective(file, "//go:embed")
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		checkDecl(pass, file, gen, embedded)
	}
}

// checkDecl reports the blank imports in decl with no comment for them.
func checkDecl(pass *analysis.Pass, file *ast.File, decl *ast.GenDecl, embedded bool) {
	comments := declComments(file, decl)
	for _, spec := range decl.Specs {
		imp, ok := spec.(*ast.ImportSpec)
		if !ok || !blank(imp.Name) {
			continue
		}
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path == "" {
			continue
		}
		if exempt(path, embedded) {
			continue
		}
		if explains(comments, path) {
			continue
		}
		pass.Reportf(imp.Pos(), "blank import of %q has no comment naming its path", path)
	}
}

// exempt reports whether path is outside the rule's subject.
func exempt(path string, embedded bool) bool {
	if path == "unsafe" {
		return true
	}
	return path == "embed" && embedded
}

// declComments returns the comments on decl and its import specs that can
// explain an import path.
func declComments(file *ast.File, decl *ast.GenDecl) []*ast.Comment {
	var (
		out []*ast.Comment
		add = func(group *ast.CommentGroup) {
			if group != nil {
				out = append(out, group.List...)
			}
		}
	)
	add(decl.Doc)
	for _, spec := range decl.Specs {
		imp, ok := spec.(*ast.ImportSpec)
		if !ok || !blank(imp.Name) {
			continue
		}
		add(imp.Doc)
		add(imp.Comment)
	}
	if !decl.Lparen.IsValid() {
		return out
	}
	last := decl.Specs[len(decl.Specs)-1]
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment.Pos() > last.End() && comment.End() < decl.Rparen {
				out = append(out, comment)
			}
		}
	}
	return out
}

// explains reports whether one of comments names path.
func explains(comments []*ast.Comment, path string) bool {
	for _, comment := range comments {
		if names(comment.Text, path) {
			return true
		}
	}
	return false
}

// names reports whether text contains path as a whole token.
func names(text, path string) bool {
	for offset := 0; ; {
		i := strings.Index(text[offset:], path)
		if i < 0 {
			return false
		}
		i += offset
		end := i + len(path)
		if beforePath(text, i) && afterPath(text, end) {
			return true
		}
		offset = i + 1
	}
}

// beforePath reports whether i is a plausible start for path in text. A path
// at the start of a comment or in a URL may follow a slash.
func beforePath(text string, i int) bool {
	if i == 0 || !pathByte(text[i-1]) {
		return true
	}
	if i == 2 && strings.HasPrefix(text, "//") {
		return true
	}
	return nearHost(text[:i])
}

// nearHost reports whether the word before i is a host name, with or without
// a scheme, followed by a slash.
func nearHost(prefix string) bool {
	if i := strings.LastIndexAny(prefix, " \t\n\r"); i >= 0 {
		prefix = prefix[i+1:]
	}
	host, ok := strings.CutSuffix(prefix, "/")
	if !ok {
		return false
	}
	if i := strings.LastIndex(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	return !strings.Contains(host, "/") && strings.Contains(host, ".")
}

// afterPath reports whether i is a plausible end for path in text. A path byte
// continues the match; a dot or hyphen ends it when no path byte follows, and
// a run of dots is an ellipsis.
func afterPath(text string, i int) bool {
	if i == len(text) {
		return true
	}
	c := text[i]
	if !pathByte(c) {
		return true
	}
	if c == '.' {
		if i+1 < len(text) && 'A' <= text[i+1] && text[i+1] <= 'Z' {
			return true
		}
		end := i
		for end < len(text) && text[end] == '.' {
			end++
		}
		if end > i+1 {
			return end == len(text) || !pathByte(text[end])
		}
	}
	if c != '.' && c != '-' {
		return false
	}
	return i+1 == len(text) || !pathByte(text[i+1])
}

// pathByte reports whether c can belong to an import path.
func pathByte(c byte) bool {
	return strings.ContainsRune("/.-_+~", rune(c)) || c >= 0x80 ||
		letter(c) || digit(c)
}

// letter reports whether c is an ASCII letter.
func letter(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// digit reports whether c is an ASCII digit.
func digit(c byte) bool { return '0' <= c && c <= '9' }

// hasDirective reports whether file carries the named //go: directive. A space
// follows the name in the directive.
func hasDirective(file *ast.File, name string) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, name+" ") {
				return true
			}
		}
	}
	return false
}

// blank reports whether id is the blank identifier.
func blank(id *ast.Ident) bool { return id != nil && id.Name == "_" }
