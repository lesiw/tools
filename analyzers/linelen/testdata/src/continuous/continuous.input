package continuous

import "fmt"

func errString() error {
	return fmt.Errorf("the quick brown fox jumps over the lazy dog and then keeps on running")
}

func shortString() error {
	return fmt.Errorf("short: %s", aFunctionWithAVeryLongNameIndeed(1, 2, 3, 4, 5)) // want "line is .* characters long, exceeds 79 limit"
}

func aFunctionWithAVeryLongNameIndeed(a, b, c, d, e int) string { return "" }

// See https://example.com/a/very/long/path/that/keeps/going/and/going/and/never/stops

// See https://example.com and then some more words to push it over the limit here // want "line is .* characters long, exceeds 79 limit"

func manyStrings() {
	_ = []string{"alphabet", "bicycle", "carousel", "dandelion", "elephant", "flamingo"} // want "line is .* characters long, exceeds 79 limit"
}

const raw = `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`

var _ = `short
` + aFunctionWithAVeryLongNameIndeed(1, 2, 3, 4, 5) // want "line is .* characters long, exceeds 79 limit"
