package a

func f(x any) int {
	switch x.(type) { // want `type switch binds nothing`
	case int:
		return x.(int)
	case string:
		return len(x.(string))
	}
	return 0
}

func ok(x any) int {
	switch v := x.(type) {
	case int:
		return v
	case string:
		return len(v)
	}
	return 0
}

func other(x, y any) int {
	switch x.(type) {
	case int:
		return y.(int)
	}
	return 0
}

func noassert(x any) int {
	switch x.(type) {
	case int:
		return 1
	}
	return 0
}

func multi(x any) int {
	switch x.(type) {
	case int, int8:
		return len(toString(x))
	}
	return 0
}

func withDefault(x any) int {
	switch x.(type) {
	case int:
		return x.(int)
	default:
		return len(toString(x))
	}
}

func toString(x any) string { return "" }
