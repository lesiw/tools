// Package regexpsimplify provides an analysis.Analyzer that reports a
// character class in a regexp pattern that has a shorter spelling.
//
// Both spellings are correct, so this is a house rule: the fix writes the
// short spelling. A class is reported when the short form parses to the same
// expression as the class, under the flags the call uses. The fix replaces the
// whole literal, so a literal that strconv.Quote would write differently is
// left alone.
//
// The rule is inspired by go-critic's regexpSimplify, which matches a fixed
// list of spellings and also rewrites repetitions, alternations, and ranges.
package regexpsimplify

import (
	"go/ast"
	"go/token"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "regexpsimplify",
	Doc:  "reports a regexp character class that has a shorter spelling",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

func check(pass *analysis.Pass, call *ast.CallExpr) {
	if len(call.Args) == 0 {
		return
	}
	flags, ok := flagsFor(pass, call)
	if !ok {
		return
	}
	lit, ok := ast.Unparen(call.Args[0]).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return
	}
	pattern, err := strconv.Unquote(lit.Value)
	if err != nil {
		return
	}
	// The edit replaces the whole literal, so an escape the change did not
	// touch would be respelled the way strconv.Quote writes it. A literal that
	// is not already spelled that way is left alone.
	if quote(lit, pattern) != lit.Value {
		return
	}
	rewritten, rewrites := simplify(pattern, flags)
	if len(rewrites) == 0 {
		return
	}
	notes := make([]string, len(rewrites))
	for i, r := range rewrites {
		notes[i] = r.class + " can be replaced by " + r.short
	}
	// The class offsets are into the unquoted pattern, and an interpreted
	// literal's escapes do not line up with its source bytes, so the edit
	// replaces the literal whole.
	pass.Report(analysis.Diagnostic{
		Pos:     lit.Pos(),
		End:     lit.End(),
		Message: strings.Join(notes, "; "),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "Write the short form",
			TextEdits: []analysis.TextEdit{{
				Pos:     lit.Pos(),
				End:     lit.End(),
				NewText: []byte(quote(lit, rewritten)),
			}},
		}},
	})
}

// patternFlags maps each function that takes a pattern as its first argument
// to the flags regexp parses that pattern with.
var patternFlags = map[string]syntax.Flags{
	"Compile":          syntax.Perl,
	"CompilePOSIX":     syntax.POSIX,
	"MustCompile":      syntax.Perl,
	"MustCompilePOSIX": syntax.POSIX,
	"Match":            syntax.Perl,
	"MatchReader":      syntax.Perl,
	"MatchString":      syntax.Perl,
}

// flagsFor returns the flags that regexp parses call's first argument with,
// and false when call does not take a pattern.
func flagsFor(pass *analysis.Pass, call *ast.CallExpr) (syntax.Flags, bool) {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "regexp" {
		return 0, false
	}
	// A Regexp value has methods with these names, and their argument is text
	// to match rather than a pattern.
	if fn.Signature().Recv() != nil {
		return 0, false
	}
	flags, ok := patternFlags[fn.Name()]
	return flags, ok
}

// quote returns s as a Go string literal, written the way lit is.
func quote(lit *ast.BasicLit, s string) string {
	if strings.HasPrefix(lit.Value, "`") {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}

type rewrite struct {
	class, short string
}

// simplify returns pattern with every class that has a shorter spelling
// replaced, and the classes it replaced.
func simplify(pattern string, flags syntax.Flags) (string, []rewrite) {
	// A pattern that does not compile is left to regexpconst.
	if _, err := syntax.Parse(pattern, flags); err != nil {
		return pattern, nil
	}
	var (
		work     = pattern
		delta    int
		rewrites []rewrite
	)
	for _, c := range classes(pattern) {
		short := shortForm(c.body, flags)
		if short == "" {
			continue
		}
		// A rewrite pulls the bytes on either side of a class together, and an
		// escape or a repetition count there can take them: \0 beside a digit
		// is one escape, and a digit inside braces is a count. The rewrite is
		// kept only when the pattern it lands in parses the same.
		start := c.start + delta
		next := work[:start] + short + work[start+c.end-c.start:]
		if !sameText(work, next, flags) {
			continue
		}
		work = next
		delta += len(short) - (c.end - c.start)
		class := pattern[c.start:c.end]
		if r := (rewrite{class, short}); !slices.Contains(rewrites, r) {
			rewrites = append(rewrites, r)
		}
	}
	return work, rewrites
}

// class is one character class of a pattern: the byte range it occupies and
// the body between its brackets.
type class struct {
	start, end int
	body       string
}

// classes returns every character class in pattern, which must parse. simplify
// checks that before it scans.
func classes(pattern string) (found []class) {
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			i += skipEscape(pattern[i:]) - 1
		case '[':
			end := closeClass(pattern, i)
			found = append(found, class{i, end, pattern[i+1 : end-1]})
			i = end - 1
		}
	}
	return found
}

// skipEscape returns the length of the escape at the start of pat, which
// begins with a backslash. It is two bytes unless a \Q...\E quote starts
// here, which runs to the end of the pattern when no \E follows: no escape
// after its first two bytes can open a class.
func skipEscape(pat string) int {
	if pat[1] == 'Q' {
		// \Q quotes everything up to \E, so what looks like a class inside
		// it is text to match.
		if end := strings.Index(pat, `\E`); end >= 0 {
			return end + 2
		}
		return len(pat)
	}
	return 2
}

// closeClass returns the offset just past the class that opens at pattern[i],
// and the end of the pattern when the class does not close.
func closeClass(pattern string, i int) int {
	j := i + 1
	// A ']' after the '[' or after a leading '^' is a character of the body,
	// not the end of the class.
	if j < len(pattern) && pattern[j] == '^' {
		j++
	}
	if j < len(pattern) && pattern[j] == ']' {
		j++
	}
	for ; j < len(pattern); j++ {
		switch pattern[j] {
		case '\\':
			// A class takes no \Q, and \] is the only escape that hides the
			// closer, so two bytes is enough.
			j++
		case '[':
			// [:name:] is one element of the body, and a bare '[' is a
			// character of it.
			if rest, ok := strings.CutPrefix(pattern[j:], "[:"); ok {
				if name, _, ok := strings.Cut(rest, ":]"); ok {
					j += len(name) + 3
				}
			}
		case ']':
			return j + 1
		}
	}
	return len(pattern)
}

// shortClasses are the Perl classes a class body can stand in for.
var shortClasses = []string{`\d`, `\D`, `\s`, `\S`, `\w`, `\W`}

// shortForm returns a shorter expression for the class with body, without the
// brackets, and "" when there is none. Every expression it returns matches the
// same strings as the class under flags.
func shortForm(body string, flags syntax.Flags) string {
	var (
		bracketed = "[" + body + "]"
		re, ok    = parse(bracketed, flags)
	)
	if !ok {
		return ""
	}
	// A class that matches a Perl class is that class, however its ranges are
	// spelled or ordered.
	for _, short := range shortClasses {
		if same(re, short, flags) {
			return short
		}
	}
	if short := oneChar(re, flags); short != "" {
		return short
	}
	// A body that begins with a backslash keeps it when the pattern parses the
	// same: [\n] is \n, and [\d\s] stays put.
	if strings.HasPrefix(body, `\`) && same(re, body, flags) {
		return body
	}
	// A negated body that names one Unicode property is its opposite:
	// [^\p{L}] is \P{L}.
	if property, ok := strings.CutPrefix(body, "^"); ok {
		if flipped, ok := flip(property); ok && same(re, flipped, flags) {
			return flipped
		}
	}
	return ""
}

// oneChar returns the character a one-character class can be written as, and
// "" when the class holds more than one character or the character cannot be
// written there: [x] is x, [\x41] is A, and [.] is \..
func oneChar(re *syntax.Regexp, flags syntax.Flags) string {
	one := re.Op == syntax.OpCharClass && len(re.Rune) == 2
	if !one || re.Rune[0] != re.Rune[1] {
		return ""
	}
	r := re.Rune[0]
	char := string(r)
	if bare(r) && same(re, char, flags) {
		return char
	}
	escaped := regexp.QuoteMeta(char)
	if strings.HasPrefix(escaped, `\`) && same(re, escaped, flags) {
		return escaped
	}
	return ""
}

// flip returns the opposite of a \p or \P escape, and false for any other
// text.
func flip(property string) (string, bool) {
	if len(property) < 3 || property[0] != '\\' {
		return "", false
	}
	switch property[1] {
	case 'p':
		return `\P` + property[2:], true
	case 'P':
		return `\p` + property[2:], true
	}
	return "", false
}

// same reports whether pattern parses under flags to the expression re, which
// parse has already canonicalized.
func same(re *syntax.Regexp, pattern string, flags syntax.Flags) bool {
	other, ok := parse(pattern, flags)
	return ok && re.Equal(other)
}

// sameText reports whether two patterns parse under flags to the same
// expression.
func sameText(left, right string, flags syntax.Flags) bool {
	l, ok := parse(left, flags)
	if !ok {
		return false
	}
	r, ok := parse(right, flags)
	return ok && l.Equal(r)
}

// parse returns the canonical form of pattern, and false when pattern does not
// parse under flags.
func parse(pattern string, flags syntax.Flags) (*syntax.Regexp, bool) {
	re, err := syntax.Parse(pattern, flags)
	if err != nil {
		return nil, false
	}
	return canonical(re), true
}

// canonical returns re with every literal written as the classes of the
// characters it matches, and nested concatenations flattened, so that a class
// and the characters it holds compare equal. A group like (?i) leaves a
// character folded and expands a class, and the two then differ although they
// match the same strings.
func canonical(re *syntax.Regexp) *syntax.Regexp {
	out := *re
	switch re.Op {
	case syntax.OpLiteral:
		var (
			folds = re.Flags & syntax.FoldCase
			parts = make([]*syntax.Regexp, len(re.Rune))
		)
		for i, r := range re.Rune {
			parts[i] = &syntax.Regexp{
				Op:    syntax.OpCharClass,
				Flags: re.Flags &^ syntax.FoldCase,
				Rune:  foldClass(r, folds != 0),
			}
		}
		if len(parts) == 1 {
			return parts[0]
		}
		return &syntax.Regexp{Op: syntax.OpConcat, Sub: parts}
	case syntax.OpCharClass:
		// The parser has already expanded a folded class into its runes.
		out.Flags &^= syntax.FoldCase
	}
	out.Sub = make([]*syntax.Regexp, len(re.Sub))
	for i, sub := range re.Sub {
		out.Sub[i] = canonical(sub)
	}
	if re.Op == syntax.OpConcat {
		var flat []*syntax.Regexp
		for _, sub := range out.Sub {
			if sub.Op == syntax.OpConcat {
				flat = append(flat, sub.Sub...)
				continue
			}
			flat = append(flat, sub)
		}
		out.Sub = flat
	}
	return &out
}

// foldClass returns the characters that a class holds for r, as ranges: the
// characters that case-fold together with it when folds is set.
func foldClass(r rune, folds bool) (ranges []rune) {
	if !folds {
		return []rune{r, r}
	}
	folded := []rune{r}
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		folded = append(folded, f)
	}
	slices.Sort(folded)
	for _, f := range folded {
		if n := len(ranges); n > 0 && ranges[n-1]+1 == f {
			ranges[n-1] = f
			continue
		}
		ranges = append(ranges, f, f)
	}
	return ranges
}

// bare reports whether one character is written as itself outside a class: it
// is ASCII, needs no escape, and is neither a space, an unprintable, nor a
// backtick, which a raw literal cannot hold.
func bare(r rune) bool {
	char := string(r)
	return r < utf8.RuneSelf && r != ' ' && r != '`' &&
		regexp.QuoteMeta(char) == char && unicode.IsPrint(r)
}
