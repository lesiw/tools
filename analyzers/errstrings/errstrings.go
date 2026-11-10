// Package errstrings provides an analysis.Analyzer that reports an error
// string that is capitalized or ends with punctuation.
//
// An error is usually printed after other context, as in "reading config: %v",
// so a capital or a full stop inside the message reads as a sentence break
// that the author did not intend. The Go review comments state the rule.
package errstrings

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "errstrings",
	Doc:  "reports a capitalized error string or one ending in punctuation",
	Run:  run,
}

// punctuation is the set of trailing characters an error string should not
// end with.
const punctuation = ".:;!,"

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			lit := message(pass, call)
			if lit == nil {
				return true
			}
			text, err := strconv.Unquote(lit.Value)
			if err != nil || text == "" {
				return true
			}
			if capitalized(text) {
				pass.Reportf(lit.Pos(), "error string starts with a capital letter")
			}
			if strings.ContainsRune(punctuation, rune(text[len(text)-1])) {
				pass.Reportf(lit.Pos(), "error string ends with punctuation")
			}
			return true
		})
	}
	return nil, nil
}

// message returns the format or message literal of an error constructor, if
// the call is one.
func message(pass *analysis.Pass, call *ast.CallExpr) *ast.BasicLit {
	if len(call.Args) == 0 {
		return nil
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok {
		return nil
	}
	switch pkg.Imported().Path() + "." + sel.Sel.Name {
	case "errors.New", "fmt.Errorf":
		return lit
	}
	return nil
}

// capitalized reports whether text begins with a word that is capitalized
// rather than an initialism.
func capitalized(text string) bool {
	first, _ := utf8.DecodeRuneInString(text)
	if !unicode.IsUpper(first) {
		return false
	}
	i := strings.IndexFunc(text, notLetter)
	if i < 0 {
		i = len(text)
	}
	word := text[:i]
	if word == "A" || word == "I" {
		return true // an article or a pronoun, not an acronym
	}
	return strings.ToUpper(word) != word
}

// notLetter reports whether r ends a word.
func notLetter(r rune) bool { return !unicode.IsLetter(r) }
