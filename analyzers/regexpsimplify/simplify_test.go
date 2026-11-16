package regexpsimplify

import (
	"regexp/syntax"
	"slices"
	"testing"
)

// TestShortForm checks the spelling the rule gives each class body under both
// flag sets. A missing expectation means the class keeps its brackets.
func TestShortForm(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		perl, posix string
	}{{
		name: "digit range",
		body: "0-9",
		perl: `\d`,
	}, {
		name: "digit escape",
		body: `\d`,
		perl: `\d`,
	}, {
		name: "not digit escape",
		body: `\D`,
		perl: `\D`,
	}, {
		name: "space escape",
		body: `\s`,
		perl: `\s`,
	}, {
		name: "not space escape",
		body: `\S`,
		perl: `\S`,
	}, {
		name: "word escape",
		body: `\w`,
		perl: `\w`,
	}, {
		name: "not word escape",
		body: `\W`,
		perl: `\W`,
	}, {
		name: "word ranges in another order",
		body: "0-9a-zA-Z_",
		perl: `\w`,
	}, {
		name: "space escapes",
		body: "\\t\\n\\f\\r ",
		perl: `\s`,
	}, {
		name: "space escapes with a vertical tab",
		body: "\\t\\n\\v\\f\\r ",
	}, {
		name: "named space",
		body: "[:space:]",
	}, {
		name:  "newline",
		body:  `\n`,
		perl:  `\n`,
		posix: `\n`,
	}, {
		name: "character outside ASCII",
		body: "é",
	}, {
		name:  "dot",
		body:  ".",
		perl:  "\\.",
		posix: "\\.",
	}, {
		name:  "opener",
		body:  "[",
		perl:  `\[`,
		posix: `\[`,
	}, {
		name:  "closer",
		body:  "]",
		perl:  "\\]",
		posix: "\\]",
	}, {
		name: "space character",
		body: " ",
	}, {
		name: "negated opposite property",
		body: `^\P{L}`,
		perl: `\p{L}`,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortForm(tt.body, syntax.Perl); got != tt.perl {
				t.Errorf("under Perl: got %q, want %q", got, tt.perl)
			}
			if got := shortForm(tt.body, syntax.POSIX); got != tt.posix {
				t.Errorf("under POSIX: got %q, want %q", got, tt.posix)
			}
		})
	}
}

// TestClasses checks the spans the scan finds, which is where the class syntax
// is read.
func TestClasses(t *testing.T) {
	tests := []struct {
		pattern string
		bodies  []string
	}{{
		pattern: `[]]`,
		bodies:  []string{`]`},
	}, {
		pattern: `[^]]`,
		bodies:  []string{`^]`},
	}, {
		pattern: `[a[:digit:]b]`,
		bodies:  []string{`a[:digit:]b`},
	}, {
		pattern: `\Q[x]\E`,
	}, {
		pattern: `\\[x]`,
		bodies:  []string{`x`},
	}, {
		pattern: `[\]]`,
		bodies:  []string{`\]`},
	}, {
		pattern: `[][]`,
		bodies:  []string{`][`},
	}, {
		pattern: `[[]][a]`,
		bodies:  []string{`[`, `a`},
	}}
	for _, tt := range tests {
		var got []string
		for _, c := range classes(tt.pattern) {
			got = append(got, c.body)
		}
		if !slices.Equal(got, tt.bodies) {
			t.Errorf("%s: got %q, want %q", tt.pattern, got, tt.bodies)
		}
	}
}
