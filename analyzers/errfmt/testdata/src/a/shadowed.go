package a

import (
	"errors"
	"fmt"
)

func shadowed(fmt string) error {
	if fmt == "" {
		return errors.New("empty format")
	}
	return errors.New("missing format")
}

func unshadowed(name string) (string, error) {
	if name == "" {
		return "", errors.New("empty name")
	}
	return fmt.Sprintf("load %s", name), nil
}
