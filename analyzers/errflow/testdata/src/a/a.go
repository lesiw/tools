package a

func returning(err error) int {
	if err != nil {
		return 0
	} else { // want `unnecessary else`
		return 1
	}
}

func breaking(xs []int) int {
	total := 0
	for _, x := range xs {
		if x < 0 {
			break
		} else { // want `unnecessary else`
			total += x
		}
	}
	return total
}

func panicking(ok bool) {
	if !ok {
		panic("not ok")
	} else { // want `unnecessary else`
		println("ok")
	}
}

func bothWays(err error, n int) int {
	if err != nil {
		if n > 0 {
			return n
		} else { // want `unnecessary else`
			return -n
		}
	} else { // want `unnecessary else`
		return 0
	}
}

func fallingThrough(err error) int {
	n := 0
	if err != nil {
		n = 1
	} else {
		n = 2
	}
	return n
}

func switchFallthrough(n int) int {
	switch n {
	case 0:
		fallthrough
	case 1:
		return 1
	}
	return 0
}

func chain(n int) int {
	if n == 0 {
		return 0
	} else if n == 1 {
		return 1
	}
	return -1
}

func fetch() (int, error) { return 0, nil }

func withInit() error {
	if v, err := fetch(); err != nil {
		return err
	} else {
		_ = v
	}
	return nil
}
