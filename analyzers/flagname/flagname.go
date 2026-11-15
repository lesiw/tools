// Package flagname provides an analysis.Analyzer that reports a flag name
// the flag package rejects.
//
// FlagSet.Var panics on a name that begins with a hyphen or contains an equal
// sign, and every function that declares a flag reaches it. Go 1.17 added the
// panic, so a name written before it may still be in the tree. A declaration
// nothing runs never panics at all, which is what the report finds.
//
// The fix drops a leading run of hyphens and ends the name at an equal sign.
// The README catalogs every check, exemption, and fix.
//
// The rule is adapted from go-critic's flagName.
package flagname

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "flagname",
	Doc:  "reports a flag name the flag package rejects",
	Run:  run,
}

// nameIndex maps each function that declares a flag to the index of its name
// parameter.
var nameIndex = map[string]int{
	"Bool":     0,
	"BoolFunc": 0,
	"Duration": 0,
	"Float64":  0,
	"Func":     0,
	"Int":      0,
	"Int64":    0,
	"String":   0,
	"Uint":     0,
	"Uint64":   0,

	// The Var forms take the value first.
	"BoolVar":     1,
	"DurationVar": 1,
	"Float64Var":  1,
	"IntVar":      1,
	"Int64Var":    1,
	"StringVar":   1,
	"TextVar":     1,
	"UintVar":     1,
	"Uint64Var":   1,
	"Var":         1,
}

// nameArg is the argument of one flag declaration that holds the name, and the
// name it holds.
type nameArg struct {
	arg  ast.Expr
	name string
}

func run(pass *analysis.Pass) (any, error) {
	var decls []nameArg
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if d, ok := declaredName(pass, call); ok {
				decls = append(decls, d)
			}
			return true
		})
	}
	// A name registered twice on one set panics, so every declaration is
	// counted before any is reported.
	counts := make(map[string]int, len(decls))
	for _, d := range decls {
		counts[tidy(d.name)]++
	}
	for _, d := range decls {
		msg, ok := message(d.name)
		if !ok {
			continue
		}
		diag := analysis.Diagnostic{
			Pos:     d.arg.Pos(),
			End:     d.arg.End(),
			Message: msg,
		}
		if fix, ok := rename(d, counts); ok {
			diag.SuggestedFixes = []analysis.SuggestedFix{fix}
		}
		pass.Report(diag)
	}
	return nil, nil
}

// declaredName returns the name argument of a call that declares a flag, and
// reports false for a call that declares nothing and for a name the type
// checker does not fold to a constant.
func declaredName(pass *analysis.Pass, call *ast.CallExpr) (nameArg, bool) {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "flag" {
		return nameArg{}, false
	}
	index, ok := nameIndex[fn.Name()]
	if !ok {
		return nameArg{}, false
	}
	// A method expression takes the set as its first argument.
	if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		selection, ok := pass.TypesInfo.Selections[sel]
		if ok && selection.Kind() == types.MethodExpr {
			index++
		}
	}
	arg := call.Args[index]
	tv := pass.TypesInfo.Types[arg]
	if tv.Value == nil {
		return nameArg{}, false
	}
	return nameArg{arg, constant.StringVal(tv.Value)}, true
}

// message returns the message for a name the flag package rejects, and reports
// false for a name it accepts.
func message(name string) (string, bool) {
	switch {
	case strings.HasPrefix(name, "-"):
		return fmt.Sprintf("flag name %q begins with a hyphen", name), true
	case strings.Contains(name, "="):
		return fmt.Sprintf("flag name %q contains an equal sign", name), true
	}
	return "", false
}

// rename returns the fix that rewrites the name as a string literal, and
// reports false when it has none to suggest: nothing left of the name, a space
// the rule has nothing to say about, or a name another declaration takes.
func rename(d nameArg, counts map[string]int) (analysis.SuggestedFix, bool) {
	lit, ok := ast.Unparen(d.arg).(*ast.BasicLit)
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	renamed := tidy(d.name)
	if renamed == "" || counts[renamed] > 1 || strings.Contains(renamed, " ") {
		return analysis.SuggestedFix{}, false
	}
	return analysis.SuggestedFix{
		Message: fmt.Sprintf("Rename the flag to %q", renamed),
		TextEdits: []analysis.TextEdit{{
			Pos:     lit.Pos(),
			End:     lit.End(),
			NewText: []byte(strconv.Quote(renamed)),
		}},
	}, true
}

// tidy returns the name the fix writes: a leading run of hyphens goes, and an
// equal sign ends the name.
func tidy(name string) string {
	name, _, _ = strings.Cut(name, "=")
	return strings.TrimLeft(name, "-")
}
