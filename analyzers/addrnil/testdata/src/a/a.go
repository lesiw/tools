package a

func equal(x int) bool {
	return &x == nil // want `an address cannot be nil`
}

func unequal(x int) bool {
	return &x != nil // want `an address cannot be nil`
}

func flippedEqual(x int) bool {
	return nil == &x // want `an address cannot be nil`
}

func flippedUnequal(x int) bool {
	return nil != &x // want `an address cannot be nil`
}

func parenthesized(x int) bool {
	return (&x) == (nil) // want `an address cannot be nil`
}

func field(s struct{ n int }) bool {
	return &s.n == nil // want `an address cannot be nil`
}

func element(xs []int) bool {
	return &xs[0] != nil // want `an address cannot be nil`
}

func indirect(p *int) bool {
	return &*p == nil // want `an address cannot be nil`
}

func unset(p *int) bool {
	return &p == nil // want `an address cannot be nil`
}

func composite() bool {
	return &struct{ n int }{} == nil // want `an address cannot be nil`
}

func nilPointer(p *int) bool {
	return p == nil
}

func notNilPointer(x int, p *int) bool {
	return &x == p
}

func convertedNil(x int) bool {
	return &x == (*int)(nil)
}

func shadowedNil(x int) bool {
	nil := (*int)(nil)
	return &x == nil
}
