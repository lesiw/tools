package a

func f(x, y int) (int, int) {
	tmp := x // want `swap through tmp; write x, y = y, x instead`
	x = y
	y = tmp
	return x, y
}

func ok(x, y int) (int, int) {
	x, y = y, x
	return x, y
}

func usedAfter(x, y int) (int, int) {
	tmp := x
	x = y
	y = tmp
	return tmp, y
}

func same(x int) int {
	tmp := x
	x = x
	x = tmp
	return x
}

func pointer(x, y *int) {
	tmp := *x
	*x = *y
	*y = tmp
}
