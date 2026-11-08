package uuid

import "fmt"

func wide(b []byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b, b, b, b, b) // want "UUID built by formatting"
}

func verbs(b []byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]) // want "UUID built by formatting"
}

func notAUUID(n int) string {
	return fmt.Sprintf("%08x-%04x-%04x", n, n, n)
}

func aDate(y, m, d int) string {
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}
