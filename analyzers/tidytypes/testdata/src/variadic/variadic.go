package variadic

// Grouping these would produce `out, src ...byte`, which is not valid Go:
// only the final parameter may be variadic.
func appendBytes(out []byte, src ...byte) []byte {
	return append(out, src...)
}
