package a

import (
	"log"
	"os"
)

func f(name string) {
	defer os.Remove(name)
	if name == "" {
		log.Fatal("empty name") // want `exit skips the deferred calls`
	}
}

func g(name string) {
	defer os.Remove(name)
	os.Exit(1) // want `exit skips the deferred calls`
}

func h(cond bool) {
	if cond {
		os.Exit(1)
	}
	defer println("later")
}

func i(name string) {
	os.Exit(1)
}

func j(name string) error {
	defer os.Remove(name)
	return nil
}
