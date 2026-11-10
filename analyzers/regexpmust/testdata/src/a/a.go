package a

import "regexp"

var re = must()

func must() *regexp.Regexp {
	re, _ := regexp.Compile(`^a+$`) // want `regexp compiled from a constant; use MustCompile`
	return re
}

var good = regexp.MustCompile(`^a+$`)

func dynamic(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}
