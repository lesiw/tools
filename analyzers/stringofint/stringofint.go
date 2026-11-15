// Package stringofint provides an analysis.Analyzer that reports a conversion
// of an integer to a string.
//
// The spec says what the conversion does
// (https://go.dev/ref/spec#Conversions): it yields "a string containing the
// UTF-8 representation of the integer", so string(65) is "A" rather than "65".
// The compiler rejects the conversion in a file that names Go 1.28 or later
// (https://go.dev/issue/3939). The digits are what the conversion almost
// always means, and a program that wants the character says so by converting
// to a rune, which this rule does not report.
//
// The fix writes the digits with strconv, adding the import where the file
// lacks one, and it changes the value the program computes. It is withheld
// where no one call writes the value, where the rewrite would not compile, and
// where a comment sits in the text the rewrite replaces.
//
// The README catalogs every check, exemption, and fix.
//
// The rule is adapted from x/tools' stringintconv and revive's string-of-int.
package stringofint

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "stringofint",
	Doc:  "reports a conversion of an integer to a string",
	Run:  run,
}

// checker holds one file of the package.
type checker struct {
	pass  *analysis.Pass
	file  *ast.File
	names map[string]string
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var (
			c     = checker{pass, file, imports.Names(pass, file)}
			stack []ast.Node
		)
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			target := c.pass.TypesInfo.Types[call.Fun]
			if !target.IsType() || !anyString(target.Type) {
				return true
			}
			kinds := typeSet(c.pass.TypesInfo.TypeOf(call.Args[0]))
			if !reported(kinds) {
				return true
			}
			needsConst := needsConstant(pass, stack, call)
			pass.Report(c.diagnose(call, kinds, needsConst))
			return true
		})
	}
	return nil, nil
}

// diagnose returns the diagnostic for one conversion, with the rewrite as a
// fix when the rewrite can be written. A conversion Go requires to be constant
// is reported alone, since a call is not a constant.
func (c checker) diagnose(call *ast.CallExpr, kinds []types.BasicKind, needsConst bool) analysis.Diagnostic {
	var (
		arg                     = call.Args[0]
		tv                      = c.pass.TypesInfo.Types[arg]
		text                    = astexpr.Text(call)
		fn, to, after, fixable  = digits(tv, kinds)
		pkg, imported, writable = c.writable(call, to)
		message                 = text + " yields a string of one rune"
		fixes                   []analysis.SuggestedFix
	)
	if fixable && writable && !needsConst {
		head, tail := pkg+"."+fn+"(", after+")"
		if to != nil {
			head += to.Name() + "("
			tail = ")" + after + ")"
		}
		// The rewrite is described from the printed form and applied by
		// position, so an argument that spans lines or holds a comment keeps
		// its text.
		if !plainString(c.pass.TypesInfo.TypeOf(call.Fun)) {
			head, tail = astexpr.Text(call.Fun)+"("+head, ")"+tail
		}
		message = text + " can be replaced by " +
			head + astexpr.Text(arg) + tail
		fixes = c.fix(call, head, tail, imported)
	}
	return analysis.Diagnostic{
		Pos:            call.Pos(),
		End:            call.End(),
		Message:        message,
		SuggestedFixes: fixes,
	}
}

// fix returns the fix that replaces the text either side of the conversion's
// argument, and adds the import of strconv unless imported reports that the
// file has one.
func (c checker) fix(call *ast.CallExpr, head, tail string, imported bool) []analysis.SuggestedFix {
	var (
		arg   = call.Args[0]
		edits = []analysis.TextEdit{{
			Pos:     call.Pos(),
			End:     arg.Pos(),
			NewText: []byte(head),
		}, {
			Pos:     arg.End(),
			End:     call.End(),
			NewText: []byte(tail),
		}}
	)
	if !imported {
		edits = append(edits, imports.Add(c.pass, c.file, "strconv"))
	}
	return []analysis.SuggestedFix{{
		Message:   "Write the digits (changes behavior)",
		TextEdits: edits,
	}}
}

// needsConstant reports whether call sits where Go requires a constant: a
// constant declaration, the length of an array type, or the index of an array
// or slice literal. A map literal's key is not one.
func needsConstant(pass *analysis.Pass, stack []ast.Node, call *ast.CallExpr) bool {
	for i, n := range slices.Backward(stack) {
		switch n := n.(type) {
		case *ast.GenDecl:
			return n.Tok == token.CONST
		case *ast.ArrayType:
			return true
		case *ast.KeyValueExpr:
			key := n.Key.Pos() <= call.Pos() && call.End() <= n.Key.End()
			if i == 0 || !key {
				return false
			}
			lit, ok := stack[i-1].(*ast.CompositeLit)
			if !ok {
				return false
			}
			switch pass.TypesInfo.TypeOf(lit).Underlying().(type) {
			case *types.Array, *types.Slice:
				return true
			}
			return false
		}
	}
	return false
}

// commented reports whether a comment overlaps the text between lo and hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}

// writable returns the name the rewrite writes for strconv, whether the file
// imports it, and whether the rewrite for call can be written.
func (c checker) writable(call *ast.CallExpr, to *types.Basic) (name string, imported, writable bool) {
	var (
		arg = call.Args[0]
		pos = call.Pos()
	)
	// The text either side of the argument is replaced, so a comment there is
	// the author's and would go with it. A comment inside the argument is
	// kept.
	if commented(c.file, pos, arg.Pos()) {
		return "", false, false
	}
	if commented(c.file, arg.End(), call.End()) {
		return "", false, false
	}
	// The conversion keeps its name around a call only where every type it
	// converts to is a string.
	if !allStrings(c.pass.TypesInfo.TypeOf(call.Fun)) {
		return "", false, false
	}
	name, ok := imports.Qualifier(c.pass, c.names, "strconv", pos)
	if !ok {
		return "", false, false
	}
	// A declaration of to would take the name of the predeclared type.
	if to != nil && !reaches(c.pass, to.Name(), pos) {
		return "", false, false
	}
	_, imported = c.names["strconv"]
	return name, imported, true
}

// plainString reports whether t is the predeclared string type, which the
// rewrite replaces whole. A conversion to another string type keeps its name.
func plainString(t types.Type) bool {
	return types.Identical(t, types.Typ[types.String])
}

// reaches reports whether name names the predeclared object of that name at
// pos, so the rewrite may write it.
func reaches(pass *analysis.Pass, name string, pos token.Pos) bool {
	return imports.Lookup(pass, name, pos) == types.Universe.Lookup(name)
}

// digits returns the strconv function that writes the value as decimal digits,
// the type the value is converted to first or nil when it already has the type
// the function takes, and the text the call writes after the argument, before
// its closing parenthesis. It reports false when the kinds in the value's set
// have no one call that fits them all, which includes a constant outside the
// range of the type the call takes.
func digits(tv types.TypeAndValue, kinds []types.BasicKind) (fn string, to *types.Basic, after string, ok bool) {
	var (
		signed, unsigned bool
		fitsInt          = true
	)
	for _, k := range kinds {
		if !isNumber(k) {
			return "", nil, "", false
		}
		if types.Typ[k].Info()&types.IsUnsigned != 0 {
			unsigned = true
			continue
		}
		signed = true
		switch k {
		case types.Int, types.Int8, types.Int16:
		case types.UntypedInt:
			// int is 32 bits on some platforms, and the call converts a
			// constant to the type it takes.
			fitsInt = fitsInt && fits(tv.Value, math.MinInt32, math.MaxInt32)
		default:
			fitsInt = false
		}
	}
	// A negative value has no digits under uint64, and a value above MaxInt64
	// has none under int64, so no one call prints both signednesses.
	if signed && unsigned {
		return "", nil, "", false
	}
	// A constant above MaxInt64 reaches this point as an untyped int, which is
	// signed, and only the unsigned call holds it.
	if tv.Value != nil && !holds(tv.Value, false) && holds(tv.Value, true) {
		unsigned = true
	}
	// The call converts the value, and the conversion has to hold it.
	if tv.Value != nil && !holds(tv.Value, unsigned) {
		return "", nil, "", false
	}
	fn, to, after = "Itoa", types.Typ[types.Int], ""
	switch {
	case unsigned:
		fn, to, after = "FormatUint", types.Typ[types.Uint64], ", 10"
	case !fitsInt:
		fn, to, after = "FormatInt", types.Typ[types.Int64], ", 10"
	}
	if types.AssignableTo(tv.Type, to) {
		to = nil
	}
	return fn, to, after, true
}

// typeSet returns the basic kinds in the type set of t. A set that holds a
// type that is not basic, and a type with no set at all, have no kinds: no
// call this rule writes fits them.
func typeSet(t types.Type) []types.BasicKind {
	if t == nil {
		return nil
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return []types.BasicKind{u.Kind()}
	case *types.Interface:
		var out []types.BasicKind
		for embedded := range u.EmbeddedTypes() {
			union, ok := embedded.(*types.Union)
			if !ok {
				kinds := typeSet(embedded)
				if len(kinds) == 0 {
					return nil
				}
				out = append(out, kinds...)
				continue
			}
			for term := range union.Terms() {
				kinds := typeSet(term.Type())
				if len(kinds) == 0 {
					return nil
				}
				out = append(out, kinds...)
			}
		}
		return out
	}
	return nil
}

// reported reports whether a conversion of a value with these kinds in its
// type set is one this rule reports: a set of integers, one of which is a
// number. A set that holds a type which is not an integer converts only some
// of its instantiations, so the conversion is not this rule's subject.
func reported(kinds []types.BasicKind) (number bool) {
	for _, k := range kinds {
		if !integer(k) {
			return false
		}
		number = number || isNumber(k)
	}
	return number
}

// anyString reports whether t's set holds a string.
func anyString(t types.Type) bool {
	return slices.ContainsFunc(typeSet(t), stringKind)
}

// allStrings reports whether every type in t's set is a string: the
// predeclared type, a type declared over one, or a type parameter whose set
// holds only strings.
func allStrings(t types.Type) bool {
	kinds := typeSet(t)
	if len(kinds) == 0 {
		return false
	}
	for _, k := range kinds {
		if !stringKind(k) {
			return false
		}
	}
	return true
}

// stringKind reports whether k is a string type.
func stringKind(k types.BasicKind) bool {
	return types.Typ[k].Info()&types.IsString != 0
}

// isNumber reports whether this rule treats a value of kind k as a number.
// types.Byte is another name for uint8 and types.Rune is another name for
// int32, so converting a character type to a string is left alone under either
// spelling of it.
func isNumber(k types.BasicKind) bool {
	switch k {
	case types.Byte, types.Rune, types.UntypedRune:
		return false
	}
	return integer(k)
}

// integer reports whether k is an integer type of any width.
func integer(k types.BasicKind) bool {
	return types.Typ[k].Info()&types.IsInteger != 0
}

// fits reports whether the integer constant v is between lo and hi.
func fits(v constant.Value, lo, hi int64) bool {
	i, exact := constant.Int64Val(constant.ToInt(v))
	return exact && lo <= i && i <= hi
}

// holds reports whether the constant v fits the type the rewrite converts to,
// which is uint64 when the value is unsigned and int64 otherwise.
func holds(v constant.Value, unsigned bool) bool {
	if unsigned {
		_, ok := constant.Uint64Val(constant.ToInt(v))
		return ok
	}
	return fits(v, math.MinInt64, math.MaxInt64)
}
