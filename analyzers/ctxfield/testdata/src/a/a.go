package a

import "context"

type good struct {
	ctx context.Context // want `context.Context in a field`
}

type alsoGood struct {
	cancel context.CancelFunc
}

type nested struct {
	inner struct {
		ctx context.Context // want `context.Context in a field`
	}
}

func (g *good) run(ctx context.Context) {}

type embedded struct {
	context.Context
}

var local = struct {
	ctx context.Context // want `context.Context in a field`
}{}
