package a

import "testing"

type fake struct{}

func (fake) DeepEqual(a, b any) bool { return a == b }

func TestShadowed(t *testing.T) {
	var reflect fake
	if !reflect.DeepEqual(1, 1) {
		t.Error("shadow")
	}
}
