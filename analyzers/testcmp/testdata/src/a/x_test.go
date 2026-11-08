package a

import (
	"reflect"
	"testing"
)

func TestX(t *testing.T) {
	a, b := []int{1}, []int{1}
	if !reflect.DeepEqual(a, b) { // want `avoid reflect.DeepEqual in tests; consider go-cmp`
		t.Errorf("got %v, want %v", a, b)
	}
}
