package a

func f() int {
	return *new(int) // want `dereferenced allocation`
}

func g() *struct{ x int } {
	return &*new(struct{ x int }) // want `dereferenced allocation`
}

func ok() int {
	return 0
}

func param[T any]() T {
	return *new(T)
}

func other(p *int) int {
	return *p
}
