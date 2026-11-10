package a

func f(xs []int) int {
	switch xs[0] {
	case xs[1], xs[2], xs[1]: // want `case repeats an earlier expression`
		return 1
	case xs[3]:
		return 2
	}
	return 0
}

func g(xs []int) int {
	switch xs[0] {
	case xs[1]:
		return 1
	case xs[1]: // want `case repeats an earlier expression`
		return 2
	}
	return 0
}

func h(xs []int) int {
	switch xs[0] {
	case xs[1], xs[2]:
		return 1
	}
	return 0
}

func i(n int) int {
	switch n {
	case 1, 2, 3:
		return 1
	}
	return 0
}

func sel(a, b chan int) int {
	select {
	case <-a:
		return 1
	case <-a:
		return 2
	}
}
