package a

import (
	"math"
	"strings"
)

func f(s string) bool {
	return strings.Contains(s, s) // want `both arguments are the same value`
}

func g(n float64) float64 {
	return math.Max(n, n) // want `both arguments are the same value`
}

func ok(s, t string) bool {
	return strings.Contains(s, t)
}

func call(next func() string) bool {
	return strings.Contains(next(), next())
}

func literals() bool {
	return strings.Contains("a", "a")
}

func other(x, y int) int {
	return min(x, x)
}
