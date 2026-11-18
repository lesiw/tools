package a

import "testing"

// Helper reports the count.
func Helper() {}

// TestN exercises [Helper](1) and [T.Count](2), which are doc links.
func TestN(t *testing.T) {
	Helper()
}
