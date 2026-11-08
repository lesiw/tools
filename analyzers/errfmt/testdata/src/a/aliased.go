package a

import e "errors"

var errAliased = e.New("aliased") // want `errors.New can be replaced by fmt.Errorf`
