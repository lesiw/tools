package a

// Deprecated: use New.
func Old() {}

// Deprecated is not the form godoc reads.
func Older() {} // want `deprecation without the "Deprecated:" prefix`

// deprecated, use New.
func Oldest() {} // want `deprecation without the "Deprecated:" prefix`

// New is the replacement.
func New() {}

// This one is not deprecated at all.
func Current() {}

// deprecatedFoo is not a deprecation note; the word is part of a name.
func deprecatedFoo() {}
