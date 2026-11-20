//go:build ignore

package a

// The build leaves this file out, and it is the only caller of what it names.
func useExcluded() int { return leftToAnother() }
