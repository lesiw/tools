package a

func f(r any) any {
	return r.(any) // want `type assertion from a type to itself`
}

func g(r interface{}) interface{} {
	return r.(interface{}) // want `type assertion from a type to itself`
}

type Reader interface{ Read() }

func h(r Reader) Reader {
	return r.(Reader) // want `type assertion from a type to itself`
}

func ok(r any) int {
	n, _ := r.(int)
	return n
}

func conv(r any) any {
	return any(r)
}
