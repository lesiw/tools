package a

import (
	"bytes"
	"io"
)

type name struct{ s string } // want `single-field struct can be replaced by its underlying type: name string`

type Reader struct{ r io.Reader }

type Tagged struct {
	S string `json:"s"`
}

type Embed struct{ bytes.Buffer }

type Pair struct{ a, b int }

type Written struct{ n int } // want `single-field struct can be replaced by its underlying type: Written int`

type counter int

type Wrapped struct{ c counter } // want `single-field struct can be replaced by its underlying type: Wrapped counter`

type ptr struct{ p *int }

func (v ptr) get() int { return *v.p }

type bare struct{ p *int }

type Key struct{ k [4]byte } // want `single-field struct can be replaced by its underlying type: Key \[4\]byte`

type Alias = struct{ s string }

type Box[T any] struct{ v T }

func local() {
	type inner struct{ n int } // want `single-field struct can be replaced by its underlying type: inner int`
	var _ inner
}

type P *int

type wrapped struct{ p P }

type pair2 struct {
	a int
	b string
}
