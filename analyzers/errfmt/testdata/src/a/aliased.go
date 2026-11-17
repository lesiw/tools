package a

import (
	e "errors"
	f "fmt"
)

var errAliased = e.New(f.Sprint("item")) // want `errors.New can be replaced by fmt.Errorf`
