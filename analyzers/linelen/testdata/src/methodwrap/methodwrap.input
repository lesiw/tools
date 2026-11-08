package methodwrap

type T struct{}

func (T) Method(someLongParameterNameThatPushesPastTheLimit string, b int) error {
	return nil
}

type I interface {
	Method(someLongParameterNameThatPushesEvenFurtherPastTheLimit string, b int) error
}

func Func(someLongParameterNameThatPushesEvenFurtherPastTheLimit string, b int) error {
	return nil
}

var _ = func(someLongParameterNameThatPushesPastTheLimit string, b int) error { // want "line is .* characters long, exceeds 79 limit"
	return nil
}
