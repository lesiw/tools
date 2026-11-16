package a

import (
	"bytes"
	"strings"
)

func f(xs []int) int {
	return xs[len(xs)] // want `index out of range`
}

func ok(xs []int) int {
	return xs[len(xs)-1]
}

func tail(s, sub string) string {
	i := strings.Index(s, sub)
	return s[i:] // want `Index can return -1`
}

func head(s, sub string) string {
	i := strings.Index(s, sub)
	return s[:i] // want `Index can return -1`
}

func tailBytes(b, sub []byte) []byte {
	i := bytes.Index(b, sub)
	return b[i:] // want `Index can return -1`
}

func headBytes(b, sub []byte) []byte {
	i := bytes.Index(b, sub)
	return b[:i] // want `Index can return -1`
}

func directTail(s, sub string) string {
	return s[strings.Index(s, sub):] // want `Index can return -1`
}

func directHead(s, sub string) string {
	return s[:strings.Index(s, sub)] // want `Index can return -1`
}

func guarded(s, sub string) string {
	i := strings.Index(s, sub)
	if i < 0 {
		return ""
	}
	return s[i:]
}

func checked(s, sub string) string {
	i := strings.Index(s, sub)
	if i >= 0 {
		return s[i:]
	}
	return ""
}

func jumped(s, sub string) string {
	i := 0
	goto use
	i = strings.Index(s, sub)
use:
	return s[i:]
}

func different(s, other, sub string) string {
	i := strings.Index(s, sub)
	return other[i:]
}

func nested(s, sub string, ok bool) string {
	i := strings.Index(s, sub)
	if ok {
		return s[i:]
	}
	return ""
}

func wrongValue(s, other, sub string) string {
	i := strings.Index(other, sub)
	return s[i:]
}

func both(s, sub string) string {
	i := strings.Index(s, sub)
	return s[0:i]
}

func inCase(s, sub string, n int) string {
	switch n {
	case 0:
		i := strings.Index(s, sub)
		return s[i:] // want `Index can return -1`
	}
	return ""
}

func inComm(s, sub string, ch <-chan int) string {
	select {
	case <-ch:
		i := strings.Index(s, sub)
		return s[i:] // want `Index can return -1`
	}
	return ""
}

func other(s string, i int) string {
	return s[i:]
}

func adjusted(s, sub string) string {
	i := strings.Index(s, sub) + 1
	return s[i:]
}

func plusOne(s, sub string) string {
	i := strings.Index(s, sub)
	return s[i+1:]
}

func directOther(s, other, sub string) string {
	return s[strings.Index(other, sub):]
}

type finder struct{}

func (finder) Index(int, int) int { return 0 }

func method(f finder, xs []int, i int) []int {
	i = f.Index(i, 0)
	return xs[i:]
}
