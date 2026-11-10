package a

import (
	"path"
	"strings"
)

func f(path string) string { // want `declaration shadows the imported package "path"`
	return path
}

func ok(s string) string {
	return path.Join(s, strings.ToLower(s))
}

func multi() {
	path, other := "", "" // want `declaration shadows the imported package "path"`
	_, _ = path, other
}

func rangeVar(xs []string) {
	for _, path := range xs { // want `declaration shadows the imported package "path"`
		_ = path
	}
}

type Field struct {
	path string
}

type Method interface {
	path() string
}

func blank() {
	_ = struct {
		_ int
	}{}
}
