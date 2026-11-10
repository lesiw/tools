package a

import (
	"bytes"
	printing "bytes" // want `bytes is imported more than once`
	"strings"
)

func f() {
	_ = bytes.Buffer{}
	_ = printing.Buffer{}
	_ = strings.TrimSpace("")
}

func g() {
	_ = bytes.Buffer{}
}

func blankDup() {}
