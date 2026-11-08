package a

import (
	re "reflect"
	"testing"
)

func TestAliasedReflect(t *testing.T) {
	a, b := []int{1}, []int{1}
	if !re.DeepEqual(a, b) { // want `avoid reflect.DeepEqual in tests; consider go-cmp`
		t.Errorf("got %v, want %v", a, b)
	}
}
