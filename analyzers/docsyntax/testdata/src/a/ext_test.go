package a_test

import (
	"testing"

	"a"
)

// TestN exercises [a.N](1), which is a doc link from outside the package.
func TestN(t *testing.T) {
	a.N()
}
