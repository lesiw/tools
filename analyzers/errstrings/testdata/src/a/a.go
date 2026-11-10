package a

import (
	"errors"
	"fmt"
)

var errBad = errors.New("Something went wrong") // want `error string starts with a capital letter`

var errStop = errors.New("not found.") // want `error string ends with punctuation`

var errBoth = errors.New("Not found.") // want `error string starts with a capital letter` `error string ends with punctuation`

func f(n int) error {
	if n < 0 {
		return fmt.Errorf("Negative count: %d", n) // want `error string starts with a capital letter`
	}
	if n == 0 {
		return fmt.Errorf("count is zero") // fine
	}
	return nil
}

var errHTTP = errors.New("HTTP status is not ok")

var errGo = errors.New("Go version is too old") // want `error string starts with a capital letter`

func g() string { return "Something went wrong" }

func article() error {
	return errors.New("A value is required") // want `error string starts with a capital letter`
}

func acronym() error {
	return errors.New("TLS handshake failed")
}
