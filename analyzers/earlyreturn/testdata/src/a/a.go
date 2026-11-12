package a

func mixed(aaaaaaaaaaaaaaaaaaaaaaaa, bbbbbbbbbbbbbbbbbbbbbbbb bool, cccccccccccccccccccccccc, dddddddddddddddddddddddd bool) bool {
	return aaaaaaaaaaaaaaaaaaaaaaaa && bbbbbbbbbbbbbbbbbbbbbbbb || // want `return chains 4 conditions; early returns decompose it`
		cccccccccccccccccccccccc && dddddddddddddddddddddddd
}

func disjoin(s string) bool {
	return contains(s, "alpha-one") && contains(s, "beta-two") || // want `return chains 4 conditions; early returns decompose it`
		contains(s, "gamma-three") && contains(s, "delta-four")
}

func conjunction(aaaaaaaaaaaaaaaaaaaaaaaa, bbbbbbbbbbbbbbbbbbbbbbbb, cccccccccccccccccccccccc, dddddddddddddddddddddddd bool) bool {
	return aaaaaaaaaaaaaaaaaaaaaaaa && bbbbbbbbbbbbbbbbbbbbbbbb &&
		cccccccccccccccccccccccc && dddddddddddddddddddddddd
}

func fits(a, b, c bool) bool {
	return a && b || c
}

func two(a, b bool) bool {
	return a || b
}

func contains(s, sub string) bool { return false }
