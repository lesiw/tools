package a

import rgx "regexp"

func aliased() {
	_, _ = rgx.Compile(`a+`) // want `the error of a constant pattern is always nil`
}
