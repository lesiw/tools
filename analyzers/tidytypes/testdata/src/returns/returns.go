package returns

// Function with redundant return types
func redundantReturns() (int, int) { // want "redundant type in result list"
	return 0, 0
}

// Function with multiple redundant return groups
func multipleReturnGroups() (int, int, string, string) { // want "redundant type in result list" "redundant type in result list"
	return 0, 0, "", ""
}

// Function with mixed return types (should not trigger)
func mixedReturns() (int, string, int) {
	return 0, "", 0
}

// Function with single return (should not trigger)
func singleReturn() int {
	return 0
}

// Function with grouped returns (should not trigger)
func groupedReturns() (x, y int, z string) {
	return 0, 0, ""
}

// Function with error pattern (should not trigger)
func errorPattern() (int, error) {
	return 0, nil
}

// Complex return types
func complexReturns() ([]int, []int, map[string]int, map[string]int) { // want "redundant type in result list" "redundant type in result list"
	return nil, nil, nil, nil
}

// Named return types with redundancy (should trigger)
func namedRedundantReturns() (x int, y int) { // want "redundant type in result list"
	return 0, 0
}

// Multiple named return groups with redundancy (should trigger)
func multipleNamedGroups() (a int, b int, c string, d string) { // want "redundant type in result list" "redundant type in result list"
	return 0, 0, "", ""
}
