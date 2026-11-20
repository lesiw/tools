package broken

func f() {
	var unread int // want `a type error reached the analyzer`
}
