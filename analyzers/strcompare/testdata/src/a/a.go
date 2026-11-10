package a

import (
	"bytes"
	"strings"
)

func f(a, b string) {
	if strings.Compare(a, b) == 0 { // want `comparison through Compare`
	}
	if strings.Compare(a, b) < 0 { // want `comparison through Compare`
	}
	if strings.Compare(a, b) > 0 { // want `comparison through Compare`
	}
	if strings.Compare(a, b) == -1 { // want `comparison through Compare`
	}
	if 0 != strings.Compare(a, b) { // want `comparison through Compare`
	}
}

func g(x, y []byte) {
	if bytes.Compare(x, y) == 0 { // want `comparison through Compare`
	}
	if bytes.Compare(x, y) < 0 {
	}
}

func ok(a, b string) bool {
	return a == b || a < b || a > b
}

func stored(a, b string) bool {
	n := strings.Compare(a, b)
	return n == 0
}

type s []string

func (s) Compare(x, y string) int { return 0 }

func other(v s, a, b string) bool {
	return v.Compare(a, b) == 0
}
