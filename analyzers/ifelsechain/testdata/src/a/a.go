package a

func three(n int) int {
	if n == 0 { // want `if-else chain of 4 branches; use a switch`
		return 0
	} else if n == 1 {
		return 1
	} else if n == 2 {
		return 2
	} else if n == 3 {
		return 3
	}
	return -1
}

func withElse(n int) int {
	if n == 0 { // want `if-else chain of 4 branches; use a switch`
		return 0
	} else if n == 1 {
		return 1
	} else if n == 2 {
		return 2
	} else {
		return -1
	}
}

func two(n int) int {
	if n == 0 {
		return 0
	} else if n == 1 {
		return 1
	}
	return -1
}

func plain(n int) int {
	if n == 0 {
		return 0
	} else {
		return -1
	}
}

func withInit(n int) int {
	if m := n * 2; m == 0 {
		return 0
	} else if m == 2 {
		return 1
	} else if m == 4 {
		return 2
	} else if m == 6 {
		return 3
	}
	return -1
}
