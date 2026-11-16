// Package errwrapnil provides an analysis.Analyzer that reports a fmt.Errorf
// call whose %w verb has a nil error operand.
//
// The fmt.Errorf documentation says a %w verb with an error operand returns
// an error whose Unwrap method returns that operand
// (https://pkg.go.dev/fmt#Errorf). With one %w verb, a nil operand makes
// that Unwrap return nil, so the operand does not contribute a cause, and the
// message contains the marker %!w(<nil>) where the cause would be. With
// several %w verbs, the error implements Unwrap() []error and the nil operand
// is omitted from the slice.
//
//	return fmt.Errorf("fetch: %w", nil)
//
// The nil check is adapted from nilerrwrap, which is inspired by the x/tools
// printf analyzer. nilerrwrap reports the nil operand in positional calls;
// this analyzer adds the fix and reports indexed and star calls. Because the
// nil case overlaps with a report from nilerrwrap, a call that both analyzers
// report has two diagnostics.
//
// The fix is offered when the operand count matches the verb count and the
// number of non-nil %w verbs that would remain is not one. A literal whose
// source is its value is edited in place; any other
// constant expression is replaced with a quoted literal. The edit removes the
// verb, its operand, and the separating comma; the surrounding message text
// stays. A fix is withheld when an edit would remove a comment or leave a
// comment separated from the code it describes. A fix can leave an import
// unused. cmd/fix runs goimports after applying the edits; another driver may
// not. A fix is also withheld when the format expression or a removed operand
// contains a dot-imported identifier, because goimports does not remove a dot
// import.
//
// If no non-nil %w verb remains, the returned error does not have an Unwrap
// method; this is the single-%w case and the all-nil case. errors.Unwrap
// returns nil before and after. When every %w verb is removed, the result of a
// type assertion or errors.As to an interface with Unwrap changes, because the
// returned error does not have an Unwrap method. Code that asserted the method
// must be updated. Removing the method is the intended effect. If at least two
// non-nil %w verbs remain, the analyzer offers the fix; the returned error has
// the same errors in the Unwrap() []error slice. If exactly one non-nil %w
// verb remains, the call is reported without a fix, because the method changes
// from Unwrap() []error to Unwrap() error.
//
// A nil operand is the predeclared nil or a conversion of nil to an interface
// type. Any other expression is not inspected for nilness; a nil variable is
// left alone because the analyzer does not follow dataflow. A typed nil
// pointer that implements error is left alone: the interface value is
// non-nil, so Unwrap returns it. A typed nil pointer that does not implement
// error is not a nil error operand; nilerrwrap reports it in the suite. A type
// parameter is left alone; the analyzer does not inspect its type set. A call
// through a function value is left alone, because the callee expression is not
// fmt.Errorf.
//
// The format must be a constant string. The operand positions of a spread call
// are unknown, so it is left alone. A call with an indexed argument or a star
// width or precision that resolves to the nil %w operand is reported without a
// fix. The analyzer does not model the remaining index and star positions, and
// a star operand can be an expression with side effects. A valid index or star
// is mapped to the operand position. For a missing %w argument, nilerrwrap
// reports positional calls in the suite; neither analyzer reports indexed or
// star calls whose %w operand is missing. A call with a nil %w operand whose
// operand count does not match the verbs is reported without a fix, because
// the operand count would still differ from the verb count. A directive that
// cannot be parsed ends the scan; a nil %w before it is reported without a
// fix.
//
// In a test file, the analyzer reports the nil operand without a fix: a test
// may deliberately check the nil marker, and the fix is withheld
// conservatively. In the suite, nilerrwrap also reports it.
package errwrapnil

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "errwrapnil",
	Doc:  "reports a fmt.Errorf %w verb with a nil error operand",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var (
			name = pass.Fset.File(file.Pos()).Name()
			test = strings.HasSuffix(name, "_test.go")
		)
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, call, test)
			}
			return true
		})
	}
	return nil, nil
}

func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, test bool) {
	if !errorf(pass, call.Fun) || len(call.Args) < 2 {
		return
	}
	if call.Ellipsis.IsValid() {
		// The operand positions of a spread call are unknown; the guard keeps
		// a later nil check from reading the slice as an operand.
		return
	}
	format, ok := constantString(pass, call.Args[0])
	if !ok {
		return
	}
	args := call.Args[1:]
	verbs, positional := formatVerbs(format, len(args))
	nilVerbs, wCount := nilOperands(pass, verbs, args)
	if len(nilVerbs) == 0 {
		return
	}
	arg := args[nilVerbs[0].index]
	d := analysis.Diagnostic{
		Pos:     arg.Pos(),
		End:     arg.End(),
		Message: "fmt.Errorf has a nil %w operand",
	}
	fixable := wCount-len(nilVerbs) != 1
	dot := removesDotImport(pass, call.Args[0], nilVerbs, args)
	if !test && positional && fixable && len(args) == len(verbs) && !dot {
		if fix, ok := fix(file, call, format, nilVerbs); ok {
			d.SuggestedFixes = []analysis.SuggestedFix{fix}
		}
	}
	pass.Report(d)
}

func nilOperands(pass *analysis.Pass, verbs []verb, args []ast.Expr) (nilVerbs []verb, wCount int) {
	for _, v := range verbs {
		if v.kind == 'w' {
			wCount++
		}
		if v.kind != 'w' || v.index < 0 || v.index >= len(args) {
			continue
		}
		if isNil(pass, args[v.index]) {
			nilVerbs = append(nilVerbs, v)
		}
	}
	return nilVerbs, wCount
}

// verb is one formatting directive, with the format value offsets of the whole
// directive and the operand it takes.
type verb struct {
	kind  byte
	index int
	start int
	end   int
}

// parsed is one directive and the argument index that follows it.
type parsed struct {
	verb       verb
	next       int
	positional bool
}

// formatVerbs returns the directives of format in source order, and reports
// whether every directive takes its operand positionally. A directive that
// cannot be parsed ends the scan.
func formatVerbs(format string, argCount int) (verbs []verb, positional bool) {
	var (
		next = 1
		pos  = true
	)
	for i := 0; i < len(format); {
		start := strings.IndexByte(format[i:], '%')
		if start < 0 {
			break
		}
		start += i
		p, ok := parseDirective(format, start, next, argCount)
		if !ok {
			return verbs, false
		}
		i = p.verb.end
		next = p.next
		if !p.positional {
			pos = false
		}
		if p.verb.kind == 0 {
			break
		}
		if p.verb.kind == '%' {
			continue
		}
		verbs = append(verbs, p.verb)
	}
	return verbs, pos
}

// parseDirective parses the directive that starts at the percent sign at
// start. It reports false when the directive cannot be parsed.
func parseDirective(format string, start, next, argCount int) (parsed, bool) {
	var (
		i          = start + 1
		positional = true
	)
	if i < len(format) && format[i] == '%' {
		return parsed{
			verb:       verb{kind: '%', index: -1, start: start, end: i + 1},
			next:       next,
			positional: true,
		}, true
	}
	for i < len(format) && isFlag(format[i]) {
		i++
	}
	var pos, ok bool
	i, next, pos, ok = parseSize(format, i, next, argCount, false)
	if !ok {
		return parsed{}, false
	}
	if !pos {
		positional = false
	}
	if i < len(format) && format[i] == '.' {
		if i+1 >= len(format) {
			return parsed{
				verb: verb{
					kind:  '.',
					index: next - 1,
					start: start,
					end:   i + 1,
				},
				next:       next + 1,
				positional: false,
			}, true
		}
		i++
		i, next, pos, ok = parseSize(format, i, next, argCount, true)
		if !ok {
			return parsed{}, false
		}
		if !pos {
			positional = false
		}
	}
	return parseVerb(format, start, i, next, argCount, positional)
}

// parseVerb parses an optional index and the verb that ends the directive.
func parseVerb(format string, start, i, next, argCount int, positional bool) (parsed, bool) {
	var (
		found, ok bool
		prev      = next
	)
	i, next, found, ok = indexAt(format, i, next)
	if !ok {
		return parsed{}, false
	}
	if found {
		positional = false
	}
	if i >= len(format) {
		return parsed{
			verb:       verb{kind: 0, index: -1, start: start, end: i},
			next:       prev,
			positional: positional,
		}, true
	}
	kind := format[i]
	i++
	if kind == '%' {
		if found && (next < 1 || next > argCount) {
			next = prev
		}
		return parsed{
			verb:       verb{kind: '%', index: -1, start: start, end: i},
			next:       next,
			positional: positional,
		}, true
	}
	if found && (next < 1 || next > argCount) {
		return parsed{
			verb:       verb{kind: kind, index: -1, start: start, end: i},
			next:       prev,
			positional: false,
		}, true
	}
	return parsed{
		verb:       verb{kind: kind, index: next - 1, start: start, end: i},
		next:       next + 1,
		positional: positional,
	}, true
}

// parseSize parses an optional index and a star or decimal width or precision.
func parseSize(format string, i, next, argCount int, precision bool) (after, arg int, positional, ok bool) {
	var (
		prev     = i
		prevNext = next
		found    bool
	)
	after, arg, found, ok = indexAt(format, i, next)
	if !ok {
		return 0, 0, false, false
	}
	if found && verbFollows(format, after) {
		return prev, prevNext, true, true
	}
	star := after < len(format) && format[after] == '*'
	if found && (arg < 1 || arg > argCount) {
		return 0, 0, false, false
	}
	if found && !star {
		if !precision {
			return 0, 0, false, false
		}
		after = skipDigits(format, after)
		return after, arg, false, true
	}
	positional = !found
	if star {
		arg++
		positional = false
		after++
	} else {
		after = skipDigits(format, after)
	}
	return after, arg, positional, true
}

func verbFollows(format string, i int) bool {
	return i < len(format) && format[i] != '*' &&
		format[i] != '.' && !isDigit(format[i])
}

// indexAt parses an optional [n] argument index at i. It returns the offset
// after the index, the new argument index, whether an index was present, and
// false when the index is malformed.
func indexAt(format string, i, next int) (after, arg int, found, ok bool) {
	if i >= len(format) || format[i] != '[' {
		return i, next, false, true
	}
	var (
		j = i + 1
		n int
	)
	for j < len(format) && isDigit(format[j]) {
		if n <= len(format) {
			n = n*10 + int(format[j]-'0')
			if n > len(format) {
				n = len(format) + 1
			}
		}
		j++
	}
	if j >= len(format) || format[j] != ']' {
		return 0, 0, false, false
	}
	return j + 1, n, true, true
}

func skipDigits(format string, i int) int {
	for i < len(format) && isDigit(format[i]) {
		i++
	}
	return i
}

func isFlag(b byte) bool {
	switch b {
	case '#', '0', '+', '-', ' ':
		return true
	}
	return false
}

func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

func removesDotImport(pass *analysis.Pass, format ast.Expr, nilVerbs []verb, args []ast.Expr) bool {
	if dotImported(pass, format) {
		return true
	}
	for _, v := range nilVerbs {
		if dotImported(pass, args[v.index]) {
			return true
		}
	}
	return false
}

func dotImported(pass *analysis.Pass, e ast.Expr) bool {
	var (
		found bool
		visit func(ast.Node) bool
	)
	visit = func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.SelectorExpr:
			ast.Inspect(n.X, visit)
			return false
		case *ast.Ident:
			obj := pass.TypesInfo.Uses[n]
			if obj != nil && obj.Pkg() != nil && obj.Pkg() != pass.Pkg {
				found = true
				return false
			}
		}
		return true
	}
	ast.Inspect(e, visit)
	return found
}

func errorf(pass *analysis.Pass, fun ast.Expr) bool {
	var obj types.Object
	switch fun := ast.Unparen(fun).(type) {
	case *ast.SelectorExpr:
		obj = pass.TypesInfo.Uses[fun.Sel]
	case *ast.Ident:
		obj = pass.TypesInfo.Uses[fun]
	}
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "fmt" {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() == nil && fn.Name() == "Errorf"
}

func constantString(pass *analysis.Pass, arg ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[arg]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// isNil reports whether e is nil at compile time. A conversion of nil to an
// interface type passes the nil through; a conversion to any other type
// produces a typed nil, which is not a nil error.
func isNil(pass *analysis.Pass, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return pass.TypesInfo.Uses[e] == types.Universe.Lookup("nil")
	case *ast.CallExpr:
		if len(e.Args) != 1 {
			return false
		}
		tv, ok := pass.TypesInfo.Types[e.Fun]
		if !ok || !tv.IsType() {
			return false
		}
		t := pass.TypesInfo.TypeOf(e)
		if t == nil {
			return false
		}
		if _, isParam := t.(*types.TypeParam); isParam {
			return false
		}
		if !isInterface(t) {
			return false
		}
		return isNil(pass, e.Args[0])
	}
	return false
}

func isInterface(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// fix returns a fix that removes the nil verbs and their operands, and reports
// false when an edit would remove a comment or leave a comment separated from
// the code it describes.
func fix(file *ast.File, call *ast.CallExpr, format string, nilVerbs []verb) (analysis.SuggestedFix, bool) {
	edit, ok := formatEdit(file, call.Args[0], format, nilVerbs)
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	argEdits, ok := removeArgs(file, call, nilVerbs)
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	return analysis.SuggestedFix{
		Message:   "Remove the nil %w verbs and their operands",
		TextEdits: append([]analysis.TextEdit{edit}, argEdits...),
	}, true
}

// formatEdit returns the edit that removes the nil verbs from the format
// expression. A literal whose source is its value is edited in place; any
// other constant expression is rewritten as a quoted literal.
func formatEdit(file *ast.File, e ast.Expr, format string, nilVerbs []verb) (analysis.TextEdit, bool) {
	lit, ok := ast.Unparen(e).(*ast.BasicLit)
	if ok && lit.Kind == token.STRING {
		if source, ok := splice(lit, format, nilVerbs); ok {
			return analysis.TextEdit{
				Pos:     lit.Pos(),
				End:     lit.End(),
				NewText: []byte(source),
			}, true
		}
	}
	if commented(file, e.Pos(), e.End()) {
		return analysis.TextEdit{}, false
	}
	newText := strconv.Quote(removeSpans(format, verbSpans(nilVerbs)))
	return analysis.TextEdit{
		Pos:     e.Pos(),
		End:     e.End(),
		NewText: []byte(newText),
	}, true
}

func splice(lit *ast.BasicLit, format string, nilVerbs []verb) (string, bool) {
	if len(lit.Value) < 2 {
		return "", false
	}
	quote := lit.Value[0]
	if quote != '`' && quote != '"' {
		return "", false
	}
	inner := lit.Value[1 : len(lit.Value)-1]
	if inner != format {
		return "", false
	}
	return string(quote) + removeSpans(inner, verbSpans(nilVerbs)) +
		string(quote), true
}

func verbSpans(nilVerbs []verb) [][2]int {
	spans := make([][2]int, len(nilVerbs))
	for i, v := range nilVerbs {
		spans[i] = [2]int{v.start, v.end}
	}
	return spans
}

func removeSpans(s string, spans [][2]int) string {
	var (
		b    []byte
		last int
	)
	for _, span := range spans {
		b = append(b, s[last:span[0]]...)
		last = span[1]
	}
	return string(append(b, s[last:]...))
}

// removeArgs returns the edits that delete the nil operands from the call. A
// run of consecutive operands shares one edit, which takes the following comma
// and whitespace, or the preceding comma when the run reaches the last
// operand.
func removeArgs(file *ast.File, call *ast.CallExpr, nilVerbs []verb) ([]analysis.TextEdit, bool) {
	indices := make([]int, len(nilVerbs))
	for i, v := range nilVerbs {
		indices[i] = v.index + 1
	}
	slices.Sort(indices)
	var edits []analysis.TextEdit
	for i := 0; i < len(indices); {
		j := i
		for j+1 < len(indices) && indices[j+1] == indices[j]+1 {
			j++
		}
		first, last := indices[i], indices[j]
		prev := call.Args[first-1]
		next := call.Rparen
		if last+1 < len(call.Args) {
			next = call.Args[last+1].Pos()
		}
		if commented(file, prev.End(), next) {
			return nil, false
		}
		var pos, end token.Pos
		if last+1 < len(call.Args) {
			pos, end = call.Args[first].Pos(), call.Args[last+1].Pos()
		} else {
			pos, end = prev.End(), call.Args[last].End()
		}
		edits = append(edits, analysis.TextEdit{Pos: pos, End: end})
		i = j + 1
	}
	return edits, true
}

func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment.Pos() < end && pos < comment.End() {
				return true
			}
		}
	}
	return false
}
