package fix

// Function with redundant parameter types
func redundantParams(x int, y int, z string) { // want "redundant type in parameter list"
}

// Function with multiple redundant groups
func multipleGroups(a int, b int, c string, d string) { // want "redundant type in parameter list" "redundant type in parameter list"
}

// Function with redundant return types
func redundantReturns() (int, int) { // want "redundant type in result list"
	return 0, 0
}

// Function with named redundant return types
func namedRedundantReturns() (x int, y int) { // want "redundant type in result list"
	return 0, 0
}

// Function literals with redundant types
var fn = func(a int, b int) {} // want "redundant type in parameter list"

// Complex types
func complexTypes(a []int, b []int) { // want "redundant type in parameter list"
}

// Mixed cases - parameters need fixing, results don't
func paramsNeedFix(x int, y int) (string, error) { // want "redundant type in parameter list"
	return "", nil
}

// Mixed cases - results need fixing, parameters don't
func resultsNeedFix(x int, y string) (int, int) { // want "redundant type in result list"
	return 0, 0
}

// Mixed cases - both parameters and results need fixing
func bothNeedFix(a int, b int) (string, string) { // want "redundant type in parameter list" "redundant type in result list"
	return "", ""
}

// Mixed cases - neither parameters nor results need fixing (already grouped or mixed types)
func neitherNeedFix(x, y int, z string) (int, string, error) {
	return 0, "", nil
}

// Complex mixed case with multiple redundant groups in both parameters and results
func complexMixed(a int, b int, c []string, d []string) (map[string]int, map[string]int, bool, bool) { // want "redundant type in parameter list" "redundant type in parameter list" "redundant type in result list" "redundant type in result list"
	return nil, nil, false, false
}

// Mixed case with some grouped, some not
func partiallyGrouped(x, y int, z int, w string) (bool, bool, error) { // want "redundant type in parameter list" "redundant type in result list"
	return false, false, nil
}

// Function with unnamed parameters and results needing fixes
func unnamedMixed(int, int, string) (bool, bool) { // want "redundant type in parameter list" "redundant type in result list"
	return false, false
}
