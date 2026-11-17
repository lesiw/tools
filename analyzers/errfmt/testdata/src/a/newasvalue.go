package a

import (
	"errors"
	"fmt"
)

var newError = errors.New

func wrapHeld(name string) error {
	if name == "" {
		return errors.New("empty name")
	}
	return fmt.Errorf("load %s: %w", name, newError("failed"))
}
