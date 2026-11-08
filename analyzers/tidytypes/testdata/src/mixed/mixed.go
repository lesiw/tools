package mixed

// Function with redundant parameter types
func redundantParams(x int, y int, z string) { // want "redundant type in parameter list"
}

// Function with redundant return types
func redundantReturns() (int, int) { // want "redundant type in result list"
	return 0, 0
}

// Function with both redundant parameters and returns
func bothRedundant(a int, b int) (string, string) { // want "redundant type in parameter list" "redundant type in result list"
	return "", ""
}

// Function with mixed parameter types (should not trigger)
func mixedParams(x int, y string, z int) {
}

// Function with mixed return types (should not trigger)
func mixedReturns() (int, string, int) {
	return 0, "", 0
}

// Function with single parameter (should not trigger)
func singleParam(x int) {
}

// Function with single return (should not trigger)
func singleReturn() int {
	return 0
}

// Function with grouped parameters (should not trigger)
func groupedParams(x, y int, z string) {
}

// Function with grouped returns (should not trigger)
func groupedReturns() (x, y int, z string) {
	return 0, 0, ""
}

// Complex function with both parameter and return redundancy
func complexBoth(a []int, b []int, c string) (map[string]int, map[string]int, error) { // want "redundant type in parameter list" "redundant type in result list"
	return nil, nil, nil
}

// Function with mixed redundancy patterns
func mixedRedundancy(x int, y, z int, w string, v string) (bool, bool, int) { // want "redundant type in parameter list" "redundant type in parameter list" "redundant type in result list"
	return false, false, 0
}
