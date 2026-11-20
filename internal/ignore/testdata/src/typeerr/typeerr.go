package typeerr

func f() {
	var unread int // want `a type error reached the analyzer`
}
