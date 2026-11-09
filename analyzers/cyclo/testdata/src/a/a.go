package a

// atLimit scores exactly the limit, so it is silent. The branches are mixed on
// purpose: a conjunction, a disjunction, both kinds of loop, a switch with a
// grouped case, and a select all count.
func atLimit(a, b, c, d, e, f, g, h bool, xs []int) (count int) {
	if a {
		count++
	}
	if b && c {
		count++
	}
	if d || e {
		count++
	}
	if f {
		count++
	}
	if g {
		count++
	}
	if h {
		count++
	}
	for range xs {
		count++
	}
	for i := 0; i < len(xs); i++ {
		count++
	}
	switch count {
	case 1, 2:
	default:
	}
	select {
	default:
	}
	if count > 0 && len(xs) > 0 {
		count++
	}
	return count
}

// overLimit is atLimit with one more branch.
func overLimit(a, b, c, d, e, f, g, h bool, xs []int) (count int) { // want `cyclomatic complexity 16 exceeds 15`
	if a {
		count++
	}
	if b && c {
		count++
	}
	if d || e {
		count++
	}
	if f {
		count++
	}
	if g {
		count++
	}
	if h {
		count++
	}
	for range xs {
		count++
	}
	for i := 0; i < len(xs); i++ {
		count++
	}
	switch count {
	case 1, 2:
	default:
	}
	select {
	default:
	}
	if count > 0 && len(xs) > 0 {
		count++
	}
	if count > 1 {
		count++
	}
	return count
}

// dispatch is a jump table: grouped cases, a default, and a fallthrough, all of
// which cost nothing beyond the switch itself.
func dispatch(op int) int {
	switch op {
	case 0:
		return 0
	case 1, 2:
		return 1
	case 3:
		fallthrough
	case 4:
		return 4
	case 5, 6, 7:
		return 7
	default:
		return -1
	}
}

// route is a channel router: receives, a send, and a default.
func route(done <-chan struct{}, out chan<- struct{}, a, b <-chan int) int {
	select {
	case <-done:
		return 0
	case v := <-a:
		return v
	case v := <-b:
		return v
	case out <- struct{}{}:
		return 1
	default:
		return -1
	}
}

// conjunction is one decision written as eight branches, and the metric charges
// for each of them.
func conjunction(a, b, c, d, e, f, g, h bool) bool {
	return a && b && c && d && e && f && g && h
}

// closure charges its branches to itself, not to the function that declares
// it. The outer function is 1; walk is 16 on its own.
func closure(a, b, c, d, e, f, g, h bool, xs []int) {
	walk := func() bool { // want `cyclomatic complexity 16 exceeds 15`
		if a && b {
			return true
		}
		if c || d {
			return true
		}
		for _, x := range xs {
			if e && f {
				return true
			}
			if x > 0 {
				return true
			}
		}
		switch {
		case g, h:
			return true
		}
		for range xs {
		}
		if !a && !b {
			return true
		}
		if g || h {
			return true
		}
		if a {
			return true
		}
		return false
	}
	_ = walk()
}
