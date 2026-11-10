package a

import "strings"

func f(xs []int) int {
	return xs[len(xs)] // want `index out of range`
}

func ok(xs []int) int {
	return xs[len(xs)-1]
}

func g(s, sub string) string {
	i := strings.Index(s, sub)
	return s[i:]
}

func guarded(s, sub string) string {
	i := strings.Index(s, sub)
	if i < 0 {
		return ""
	}
	return s[i:]
}

func other(s string, i int) string {
	return s[i:]
}

func selector(b interface{ Index(int, int) int }, x int) int {
	return x
}
