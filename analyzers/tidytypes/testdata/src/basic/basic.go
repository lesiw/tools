package basic

// Function with redundant parameter types
func redundantParams(x int, y int, z string) { // want "redundant type in parameter list"
}

// Function with mixed parameter types (should not trigger)
func mixedParams(x int, y string, z int) {
}

// Function with single parameter (should not trigger)
func singleParam(x int) {
}

// Function with grouped parameters (should not trigger)
func groupedParams(x, y int, z string) {
}
