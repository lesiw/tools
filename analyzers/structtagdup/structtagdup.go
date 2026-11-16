// Package structtagdup provides an analysis.Analyzer that reports a struct tag
// that names the same key twice.
//
// The reflect package describes a struct tag as "a concatenation of optionally
// space-separated key:"value" pairs" (https://pkg.go.dev/reflect#StructTag).
// reflect.StructTag.Lookup returns "the value associated with key in the tag
// string" (https://pkg.go.dev/reflect#StructTag.Lookup), so it does not read
// the later pair.
//
//	type item struct {
//		Name string `json:"name" json:"label"`
//	}
//
// The rule reports every pair after the first with the same key. Keys are
// compared byte for byte after the tag literal is decoded; the values do not
// matter. The rule reports the second and third pairs of a tag that names the
// same key three times. Parsing stops at the first malformed pair, so a tag
// whose malformed pair comes before the repeat, or where the repeat itself is
// malformed, is left alone. A repeated key of choice, optional-value, or
// default is left alone in a package that imports
// github.com/jessevdk/go-flags, where a repeated key holds one value per
// alternative.
//
// The fix removes the later pair and the whitespace before it. The analyzer
// edits the tag literal in place, so it does not re-quote a value that holds
// an escaped quote or a non-ASCII character. A tag whose literal cannot be
// decoded is left alone.
//
// The README catalogs every check and exemption. The rule is adapted from
// staticcheck's SA5008, which reports a duplicate key among its other struct
// tag checks.
package structtagdup

import (
	"go/ast"
	"go/token"
	"strconv"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "structtagdup",
	Doc:  "reports a struct tag that names the same key twice",
	Run:  run,
}

// goFlagsKeys may appear more than once in a go-flags tag, where each value is
// one alternative.
var goFlagsKeys = map[string]struct{}{
	"choice":         {},
	"default":        {},
	"optional-value": {},
}

func run(pass *analysis.Pass) (any, error) {
	goFlags := importsGoFlags(pass)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range st.Fields.List {
				if field.Tag != nil {
					check(pass, field.Tag, goFlags)
				}
			}
			return true
		})
	}
	return nil, nil
}

// importsGoFlags reports whether any file of the package imports go-flags.
func importsGoFlags(pass *analysis.Pass) bool {
	for _, file := range pass.Files {
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err == nil && path == "github.com/jessevdk/go-flags" {
				return true
			}
		}
	}
	return false
}

// tagPair is one key:"value" pair, with the key and the bounds of the pair in
// the decoded tag.
type tagPair struct {
	key   string
	start int
	end   int
}

// tagByte is the source span that produced one byte of the decoded tag,
// relative to the start of the literal.
type tagByte struct {
	start int
	end   int
}

// check reports each pair in tag that repeats an earlier key.
func check(pass *analysis.Pass, lit *ast.BasicLit, goFlags bool) {
	tag, positions, ok := decode(pass, lit)
	if !ok {
		return
	}
	pairs := parseTag(tag)
	seen := make(map[string]struct{}, len(pairs))
	for i, pair := range pairs {
		if _, ok := seen[pair.key]; !ok {
			seen[pair.key] = struct{}{}
			continue
		}
		if _, ok := goFlagsKeys[pair.key]; goFlags && ok {
			continue
		}
		// The edit starts at the end of the previous pair, so the
		// whitespace before this pair goes too.
		var (
			start = positions[pairs[i-1].end-1].end
			end   = positions[pair.end-1].end
		)
		pass.Report(analysis.Diagnostic{
			Pos: lit.Pos() + token.Pos(positions[pair.start].start),
			End: lit.Pos() + token.Pos(end),
			Message: "struct tag key " + strconv.Quote(pair.key) +
				" is repeated; the first value is the one read",
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: "Drop the later " + strconv.Quote(pair.key) + " pair",
				TextEdits: []analysis.TextEdit{{
					Pos: lit.Pos() + token.Pos(start),
					End: lit.Pos() + token.Pos(end),
				}},
			}},
		})
	}
}

// parseTag returns the well-formed key:"value" pairs of tag. Parsing stops at
// the first malformed pair.
func parseTag(tag string) (pairs []tagPair) {
	for i := 0; i < len(tag); {
		i = skipSpaces(tag, i)
		if i >= len(tag) {
			break
		}
		key, next, ok := scanKey(tag, i)
		if !ok {
			break
		}
		end, ok := scanValue(tag, next+2)
		if !ok {
			break
		}
		if _, err := strconv.Unquote(tag[next+1 : end]); err != nil {
			break
		}
		pairs = append(pairs, tagPair{key: key, start: i, end: end})
		i = end
	}
	return
}

// skipSpaces returns the first index at or after i that is not a space.
func skipSpaces(tag string, i int) int {
	for i < len(tag) && tag[i] == ' ' {
		i++
	}
	return i
}

// scanKey returns the key that starts at start and the index of its colon.
func scanKey(tag string, start int) (string, int, bool) {
	i := start
	for i < len(tag) && keyByte(tag[i]) {
		i++
	}
	if i == start || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
		return "", 0, false
	}
	return tag[start:i], i, true
}

// keyByte reports whether b can occur in a tag key.
func keyByte(b byte) bool {
	return b > ' ' && b != ':' && b != '"' && b != 0x7f
}

// scanValue returns the index after the quoted value that starts at i.
func scanValue(tag string, i int) (int, bool) {
	for i < len(tag) && tag[i] != '"' {
		if tag[i] == '\\' {
			i++
		}
		i++
	}
	if i >= len(tag) {
		return 0, false
	}
	return i + 1, true
}

// decode returns the string value of lit and the source span of each byte of
// that value, relative to the start of the literal.
func decode(pass *analysis.Pass, lit *ast.BasicLit) (string, []tagByte, bool) {
	var (
		raw       = lit.Value
		b         []byte
		positions []tagByte
	)
	if len(raw) < 2 {
		return "", nil, false
	}
	if raw[0] == '`' {
		if len(raw) != int(lit.End()-lit.Pos()) {
			return decodeRaw(pass, lit)
		}
		tag := raw[1 : len(raw)-1]
		positions = make([]tagByte, len(tag))
		for i := range positions {
			positions[i] = tagByte{start: i + 1, end: i + 2}
		}
		return tag, positions, true
	}
	if raw[0] != '"' {
		return "", nil, false
	}
	for i := 1; i < len(raw)-1; {
		start := i
		r, multi, tail, err := strconv.UnquoteChar(raw[i:], '"')
		if err != nil {
			return "", nil, false
		}
		i += len(raw[i:]) - len(tail)
		n := 1
		if multi {
			var buf [utf8.UTFMax]byte
			n = utf8.EncodeRune(buf[:], r)
			b = append(b, buf[:n]...)
		} else {
			b = append(b, byte(r))
		}
		// Every byte of one rune maps to the rune's source span.
		for range n {
			positions = append(positions, tagByte{start: start, end: i})
		}
	}
	return string(b), positions, true
}

// decodeRaw handles a raw literal whose token text is shorter than its source,
// which happens when the scanner discards a carriage return.
func decodeRaw(pass *analysis.Pass, lit *ast.BasicLit) (string, []tagByte, bool) {
	var (
		b         []byte
		positions []tagByte
		file      = pass.Fset.File(lit.Pos())
	)
	if file == nil {
		return "", nil, false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return "", nil, false
	}
	start, end := file.Offset(lit.Pos()), file.Offset(lit.End())
	if start < 0 || end > len(src) || end-start < 2 || src[start] != '`' {
		return "", nil, false
	}
	for i := start + 1; i < end-1; i++ {
		if src[i] == '\r' {
			continue
		}
		b = append(b, src[i])
		positions = append(positions, tagByte{
			start: i - start,
			end:   i - start + 1,
		})
	}
	return string(b), positions, true
}
