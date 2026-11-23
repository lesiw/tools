package a

import "encoding/xml"

type mixed struct{}

func (m *mixed) one() {}

func (m mixed) two() {} // want `methods of mixed mix pointer and value receivers`

type pointerOnly struct{}

func (p *pointerOnly) one() {}

func (p *pointerOnly) two() {}

type valueOnly struct{}

func (v valueOnly) one() {}

func (v valueOnly) two() {}

type many struct{}

func (m *many) one() {}

func (m many) two() {} // want `methods of many mix pointer and value receivers`

func (m *many) three() {}

// A type with value methods everywhere and pointer decoders is not mixed: a
// decoder is called through a pointer and through nothing else.
type stamp struct{}

func (s stamp) one() {}

func (s stamp) two() {}

func (s *stamp) UnmarshalJSON(b []byte) error { return nil }

func (s *stamp) UnmarshalText(b []byte) error { return nil }

func (s *stamp) UnmarshalBinary(b []byte) error { return nil }

func (s *stamp) GobDecode(b []byte) error { return nil }

func (s *stamp) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error { return nil }

func (s *stamp) Scan(src any) error { return nil }

type lookalike struct{}

func (l lookalike) one() {}

func (l *lookalike) UnmarshalJSON(s string) error { return nil } // want `methods of lookalike mix pointer and value receivers`

// A name that only begins with Unmarshal is not exempt either.
func (l *lookalike) UnmarshalYAML(b []byte) error { return nil }

// A decoder on a mixed type does not excuse the other pointer methods.
type partly struct{}

func (p partly) one() {}

func (p *partly) UnmarshalJSON(b []byte) error { return nil }

func (p *partly) two() {} // want `methods of partly mix pointer and value receivers`

type pair[K comparable, V any] struct {
	k K
	v V
}

func (p pair[K, V]) one() {}

func (p *pair[K, V]) two() {} // want `methods of pair mix pointer and value receivers`

type triple[A any, B any, C any] struct{}

func (t triple[A, B, C]) one() {}

func (t *triple[A, B, C]) two() {} // want `methods of triple mix pointer and value receivers`

type box[T any] struct{ v T }

func (b box[T]) one() {}

func (b *box[T]) two() {} // want `methods of box mix pointer and value receivers`

// A generic type whose pointer methods are all decoders is not mixed.
type wrapper[T any] struct{ v T }

func (w wrapper[T]) one() {}

func (w *wrapper[T]) UnmarshalJSON(b []byte) error { return nil }
