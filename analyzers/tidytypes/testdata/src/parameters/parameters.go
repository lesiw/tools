package functions

// Function literals with redundant types
var fn1 = func(a int, b int) {} // want "redundant type in parameter list"

var fn2 = func(x string, y string, z string) {} // want "redundant type in parameter list" "redundant type in parameter list"

var fn3 = func(x int, y, z int) {} // want "redundant type in parameter list"

// Function with multiple redundant groups
func multipleGroups(a int, b int, c string, d string) { // want "redundant type in parameter list" "redundant type in parameter list"
}

// Complex types
func complexTypes(a []int, b []int, c map[string]int, d map[string]int) { // want "redundant type in parameter list" "redundant type in parameter list"
}

// Pointer types
func pointerTypes(a *int, b *int) { // want "redundant type in parameter list"
}

// Interface types
func interfaceTypes(a interface{}, b interface{}) { // want "redundant type in parameter list"
}

// Interface aliases
func interfaceAliases(a any, b any) { // want "redundant type in parameter list"
}

// Simple redundant types - should be flagged
func simpleRedundant(x int, y int) {} // want "redundant type in parameter list"

// Already grouped - should NOT be flagged
func alreadyGrouped(x, y int) {}

// Mixed - single then grouped - should flag the single one
func singleThenGrouped(x int, y, z int) {} // want "redundant type in parameter list"

// Mixed - grouped then single - should flag the single one
func groupedThenSingle(x, y int, z int) {} // want "redundant type in parameter list"

// Multiple groups - should flag only first redundant type
func multipleGroups2(x, y int, z, a int) {} // want "redundant type in parameter list"

// Complex mixed types - should flag appropriately
func complexMixed(a int, b, c int, d string, e string) {} // want "redundant type in parameter list" "redundant type in parameter list"

// All separate parameters - should flag consecutive same types
func allSeparate(a int, b int, c string, d string, e int) {} // want "redundant type in parameter list" "redundant type in parameter list"

// Unnamed parameters - should be flagged
func unnamedParams(int, int, string) {} // want "redundant type in parameter list"

// Mixed named and unnamed - multiple unnamed redundant
func mixedNamedUnnamed(int, int, string) {} // want "redundant type in parameter list"

// Multiline - first type should get flagged, but not the second
func multline(
	x int, // want "redundant type in parameter list"
	y int,
) {
}
