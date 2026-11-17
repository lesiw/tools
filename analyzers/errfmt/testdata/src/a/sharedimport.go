package a

import (
	"errors"
	"fmt"
)

var errMissing = errors.New("missing")

func label(err error) string {
	if errors.Is(err, errMissing) {
		return fmt.Sprintf("lookup: %v", err)
	}
	return ""
}
