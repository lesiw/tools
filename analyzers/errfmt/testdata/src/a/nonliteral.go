package a

import (
	"errors"
	"fmt"
)

const message = "some message"

var held = message

func fromLiteral() error {
	return errors.New("some message")
}

func fromConstant() error {
	return errors.New(message)
}

func fromVariable(msg string) error {
	return errors.New(msg)
}

func fromHeld() error {
	return errors.New(held)
}

func fromJoin() error {
	return errors.New("some message" + " tail")
}

func fromSprintf(msg string) error {
	return errors.New(fmt.Sprintf("some message: %s", msg))
}
