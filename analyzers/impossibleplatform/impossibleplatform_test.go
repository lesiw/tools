package impossibleplatform

import (
	"cmp"
	"embed"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// platformCase is one comparison under one set of build constraints. A case
// with no want reports nothing, and a case with a want and no fix reports
// without offering an edit.
type platformCase struct {
	name string
	file string // the name the build sees
	src  string
	want string // the target the diagnostic names, with the value
	fix  string // the file after the fix
}

const (
	tenTags    = "windows && t1 && t2 && t3 && t4 && t5 && t6 && t7 && t8 && t9 && t10"
	elevenTags = tenTags + " && t11"
)

// platformCases covers the constraints a fixture cannot hold: one that
// excludes the host keeps the file out of the package, so the loader never
// offers it, and a name for a platform the host is not has no file to load.
var platformCases = []platformCase{{
	name: "name gives the platform",
	file: "a_windows.go",
	src:  compare(`runtime.GOOS == "linux"`, ""),
	want: "GOOS=linux",
	fix:  compare("false", ""),
}, {
	name: "name gives the platform that is compared",
	file: "a_windows.go",
	src:  compare(`runtime.GOOS == "windows"`, ""),
}, {
	name: "name gives an architecture",
	file: "a_arm64.go",
	src:  compare(`runtime.GOARCH == "amd64"`, ""),
	want: "GOARCH=amd64",
	fix:  compare("false", ""),
}, {
	name: "name gives both",
	file: "a_linux_arm64.go",
	src:  compare(`runtime.GOARCH == "amd64"`, ""),
	want: "GOARCH=amd64",
	fix:  compare("false", ""),
}, {
	name: "name gives the operating system beside an architecture",
	file: "a_linux_arm64.go",
	src:  compare(`runtime.GOOS == "windows"`, ""),
	want: "GOOS=windows",
	fix:  compare("false", ""),
}, {
	name: "name carrying the test suffix",
	file: "a_windows_test.go",
	src:  compare(`runtime.GOOS == "plan9"`, ""),
	want: "GOOS=plan9",
	fix:  compare("false", ""),
}, {
	name: "no constraint",
	file: "a.go",
	src:  compare(`runtime.GOOS == "windows"`, ""),
}, {
	name: "suffix that is not a platform",
	file: "a_unix.go",
	src:  compare(`runtime.GOOS == "windows"`, ""),
}, {
	name: "architecture the name pairs with",
	file: "a_linux_mips.go",
	src:  compare(`runtime.GOARCH == "mips64"`, ""),
	want: "GOARCH=mips64",
	fix:  compare("false", ""),
}, {
	name: "line gives the platform",
	file: "a.go",
	src:  compare(`runtime.GOOS == "linux"`, "windows"),
	want: "GOOS=linux",
	fix:  compare("false", "windows"),
}, {
	name: "line and name agree",
	file: "a_linux.go",
	src:  compare(`runtime.GOOS == "windows"`, "linux"),
	want: "GOOS=windows",
	fix:  compare("false", "linux"),
}, {
	name: "either of two platforms",
	file: "a.go",
	src:  compare(`runtime.GOOS == "plan9"`, "linux || windows"),
	want: "GOOS=plan9",
	fix:  compare("false", "linux || windows"),
}, {
	name: "one of the two platforms compared",
	file: "a.go",
	src:  compare(`runtime.GOOS == "windows"`, "linux || windows"),
}, {
	name: "negation",
	file: "a.go",
	src:  compare(`runtime.GOOS == "windows"`, "!windows"),
	want: "GOOS=windows",
	fix:  compare("false", "!windows"),
}, {
	name: "platform the tag also covers",
	file: "a.go",
	src:  compare(`runtime.GOOS == "android"`, "linux"),
}, {
	name: "platform the tag does not cover",
	file: "a.go",
	src:  compare(`runtime.GOOS == "plan9"`, "linux"),
	want: "GOOS=plan9",
	fix:  compare("false", "linux"),
}, {
	name: "unix tag",
	file: "a.go",
	src:  compare(`runtime.GOOS == "windows"`, "unix"),
	want: "GOOS=windows",
	fix:  compare("false", "unix"),
}, {
	name: "unix tag covering a value",
	file: "a.go",
	src:  compare(`runtime.GOOS == "darwin"`, "unix"),
}, {
	name: "free tag",
	file: "a.go",
	src:  compare(`runtime.GOOS == "windows"`, "alpha"),
}, {
	name: "free tag beside a platform",
	file: "a.go",
	src:  compare(`runtime.GOOS == "linux"`, "windows && alpha"),
	want: "GOOS=linux",
	fix:  compare("false", "windows && alpha"),
}, {
	name: "free tag an or puts in reach",
	file: "a.go",
	src:  compare(`runtime.GOOS == "windows"`, "windows || alpha"),
}, {
	name: "value the go command does not know",
	file: "a.go",
	src:  compare(`runtime.GOOS == "qnx"`, "windows"),
}, {
	name: "signed comparison",
	file: "a.go",
	src:  compare(`runtime.GOOS < "windows"`, "windows"),
}, {
	name: "architecture the platform leaves open",
	file: "a.go",
	src:  compare(`runtime.GOARCH == "arm"`, "windows"),
}, {
	name: "named constant",
	file: "a.go",
	src: "//go:build windows\n\npackage a\n\nimport \"runtime\"\n\n" +
		"const other = \"linux\"\n\nfunc f() bool {\n" +
		"\treturn runtime.GOOS == other\n}\n",
}, {
	name: "dot import",
	file: "a.go",
	src: "//go:build windows\n\npackage a\n\nimport . \"runtime\"\n\n" +
		"func f() bool {\n\treturn GOOS == \"linux\"\n}\n",
	want: "GOOS=linux",
	fix: "//go:build windows\n\npackage a\n\nimport . \"runtime\"\n\n" +
		"func f() bool {\n\treturn false\n}\n",
}, {
	name: "comparison of the two",
	file: "a.go",
	src:  compare("runtime.GOOS == runtime.GOARCH", "windows"),
}, {
	name: "literal on the left",
	file: "a.go",
	src:  compare(`"linux" == runtime.GOOS`, "windows"),
	want: "GOOS=linux",
	fix:  compare("false", "windows"),
}, {
	name: "not equal",
	file: "a.go",
	src:  compare(`runtime.GOARCH != "wasm"`, "!wasm"),
	want: "GOARCH=wasm",
	fix:  compare("true", "!wasm"),
}, {
	name: "comment inside the comparison",
	file: "a.go",
	src:  compare(`runtime.GOOS /* note */ == "linux"`, "windows"),
	want: "GOOS=linux",
}, {
	name: "legacy line",
	file: "a.go",
	src:  compare(`runtime.GOOS == "linux"`, "", "// +build windows"),
	want: "GOOS=linux",
	fix:  compare("false", "", "// +build windows"),
}, {
	name: "legacy line the package clause follows",
	file: "a.go",
	src: "// +build windows\npackage a\n\nimport \"runtime\"\n\n" +
		"func f() bool {\n\treturn runtime.GOOS == \"linux\"\n}\n",
}, {
	name: "legacy line after a block comment",
	file: "a.go",
	src: "/* note */\n\n// +build windows\n\npackage a\n\n" +
		"import \"runtime\"\n\n" +
		"func f() bool {\n\treturn runtime.GOOS == \"linux\"\n}\n",
}, {
	name: "build line the package clause follows",
	file: "a.go",
	src: "//go:build windows\npackage a\n\nimport \"runtime\"\n\n" +
		"func f() bool {\n\treturn runtime.GOOS == \"linux\"\n}\n",
}, {
	name: "legacy lines the build line overrides",
	file: "a.go",
	src: compare(`runtime.GOOS == "linux"`, "windows",
		"// +build linux"),
	want: "GOOS=linux",
	fix:  compare("false", "windows", "// +build linux"),
}, {
	name: "more free tags than the rule assigns",
	file: "a.go",
	src:  compare(`runtime.GOOS == "linux"`, elevenTags),
}, {
	name: "free tags within the limit",
	file: "a.go",
	src:  compare(`runtime.GOOS == "linux"`, tenTags),
	want: "GOOS=linux",
	fix:  compare("false", tenTags),
}}

// TestConstraints drives the rule over the cases in the table, one file at a
// time, with the file name and the constraint as inputs.
func TestConstraints(t *testing.T) {
	imp := importer.Default()
	for _, tt := range platformCases {
		t.Run(tt.name, func(t *testing.T) {
			fset, reported := analyze(t, imp, tt.file, tt.src)
			if tt.want == "" {
				for _, d := range reported {
					t.Errorf("reported %q, want nothing", d.Message)
				}
				return
			}
			if len(reported) != 1 {
				t.Fatalf("got %d diagnostics, want 1", len(reported))
			}
			got := reported[0].Message
			want := "the file's build constraints rule out " + tt.want
			if got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
			fixes := reported[0].SuggestedFixes
			if tt.fix == "" {
				if len(fixes) != 0 {
					t.Errorf("got %d fixes, want none", len(fixes))
				}
				return
			}
			if len(fixes) != 1 {
				t.Fatalf("got %d fixes, want 1", len(fixes))
			}
			if got := apply(tt.src, fset, fixes[0]); got != tt.fix {
				t.Errorf("fix =\n%q\nwant:\n%q", got, tt.fix)
			}
		})
	}
}

// compare returns a file whose body is the comparison under the constraint. A
// condition is written as it appears after //go:build, and a comment that is
// not a condition is passed whole.
func compare(expr, condition string, comments ...string) string {
	var head string
	if condition != "" {
		comments = append(comments, "//go:build "+condition)
	}
	for _, comment := range comments {
		head += comment + "\n"
	}
	if head != "" {
		head += "\n"
	}
	return head + "package a\n\nimport \"runtime\"\n\n" +
		"func f() bool {\n\treturn " + expr + "\n}\n"
}

// analyze parses and type-checks one file under name and returns what the rule
// reported, with the file set that locates it.
func analyze(t *testing.T, imp types.Importer, name, src string) (fset *token.FileSet, reported []analysis.Diagnostic) {
	t.Helper()
	var (
		file *ast.File
		info = &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Uses:  make(map[*ast.Ident]types.Object),
		}
		err error
	)
	fset = token.NewFileSet()
	file, err = parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: imp}
	if _, err := conf.Check("a", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	pass := &analysis.Pass{
		Analyzer:  Analyzer,
		Fset:      fset,
		Files:     []*ast.File{file},
		TypesInfo: info,
	}
	pass.Report = func(d analysis.Diagnostic) {
		reported = append(reported, d)
	}
	pass.ReadFile = func(string) ([]byte, error) { return []byte(src), nil }
	if _, err := run(pass); err != nil {
		t.Fatal(err)
	}
	return fset, reported
}

// apply returns src with a fix's edits made, last one first so that each
// offset still addresses the bytes it was measured against.
func apply(src string, fset *token.FileSet, fix analysis.SuggestedFix) string {
	edits := slices.Clone(fix.TextEdits)
	if len(edits) == 0 {
		return src
	}
	slices.SortFunc(edits, func(x, y analysis.TextEdit) int {
		return cmp.Compare(y.Pos, x.Pos)
	})
	file := fset.File(edits[0].Pos)
	for _, edit := range edits {
		src = src[:file.Offset(edit.Pos)] + string(edit.NewText) +
			src[file.Offset(edit.End):]
	}
	return src
}
