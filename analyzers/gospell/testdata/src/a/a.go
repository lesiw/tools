package a

import "fmt"

// The context is cancelled when the request ends. // want "spelled canceled$"
func cancelled() {}

// This behaviour is deliberate. // want "spelled behavior$"
func behaviour() string {
	return "the colour of it" // want "spelled color$"
}

// Whilst the loop runs. // want "spelled While$"
func whilst() {}

// An identifier is not prose, so a field with the other spelling is left
// alone.
type t struct {
	cancelled bool
}

func fine() string {
	// An indented block is code:
	//
	//	cancelled := true
	return "canceled"
}

// An example's expected output is the program's text, not the author's.
func ExampleOutput() {
	fmt.Println("ahoj vyvojari golang")
	// Output:
	// ahoj vyvojari golang
}
