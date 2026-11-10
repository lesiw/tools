package a

func f(cond bool) {
	if cond {
		println("same")
	} else { // want `both branches hold the same statements`
		println("same")
	}
}

func g(cond bool) {
	if cond {
		println("yes")
	} else {
		println("no")
	}
}

func h(cond, other bool) {
	if cond {
		println("same")
	} else if other {
		println("same")
	}
}

func i(cond bool) {
	if cond {
	} else {
	}
}
