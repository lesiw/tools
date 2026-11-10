package a

func f() []int {
	var xs []int
	xs = append(xs, 1) // want `2 appends to one slice`
	xs = append(xs, 2)
	return xs
}

func ok() []int {
	var xs []int
	xs = append(xs, 1, 2)
	return xs
}

func different() ([]int, []int) {
	var xs, ys []int
	xs = append(xs, 1)
	ys = append(ys, 2)
	return xs, ys
}

func broken() []int {
	var xs []int
	xs = append(xs, 1)
	println("between")
	xs = append(xs, 2)
	return xs
}

func spread(xs, ys, zs []int) []int {
	xs = append(xs, ys...)
	xs = append(xs, zs...)
	return xs
}

// A comment between the appends explains one of them, and merging the calls
// would separate it from the line it was written for.
func commented() []string {
	var argv []string
	// Only pass Bsymbolic on non-Windows.
	argv = append(argv, "-Bsymbolic")
	// Mark the shared library non-closeable.
	argv = append(argv, "-nodelete")
	return argv
}
