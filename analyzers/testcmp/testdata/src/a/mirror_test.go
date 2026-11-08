package a

import (
	reflect "mirror"
	"testing"
)

func TestOtherPackage(t *testing.T) {
	if !reflect.DeepEqual(1, 1) {
		t.Error("mirror")
	}
}
