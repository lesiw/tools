package a

import "bytes"

func f(n int) []byte {
	return bytes.Repeat([]byte{0}, n) // want `zero bytes from bytes.Repeat`
}

func ok(n int) []byte {
	return make([]byte, n)
}

func g(n int) []byte {
	return bytes.Repeat([]byte{1}, n)
}

func h(n int) []byte {
	return bytes.Repeat([]byte{0, 1}, n)
}
