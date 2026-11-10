package a

type client struct{}

func (c *client) get() {}

func (c *client) put() {}

func (cl *client) post() {} // want `receiver name "cl" differs from "c"`

type other struct{}

func (o other) one() {}

func (o *other) two() {}

func (this *other) three() {} // want `receiver "this"; use the type's initial`

type quiet struct{}

func (quiet) zero() {}

func (q *quiet) one() {}

type box[T any] struct{}

func (b *box[T]) get() {}

func (b *box[T]) put() {}

// The odd name out is the one reported, whichever order the file is in.
type bulk struct{}

func (z *bulk) a() {}

func (z *bulk) b() {}

func (z *bulk) c() {}

func (x *bulk) d() {} // want `receiver name "x" differs from "z"`
