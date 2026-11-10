package a

import "context"

func first(ctx context.Context, n int) {}

func second(n int, ctx context.Context) { // want `context.Context is not the first parameter`
}

func none(n int) {}

func (s *server) handle(ctx context.Context, n int) {}

func (s *server) wrong(n int, ctx context.Context) { // want `context.Context is not the first parameter`
}

func named(ctx context.Context, a, b string) {}

type Ctx = context.Context

func aliased(n int, ctx Ctx) { // want `context.Context is not the first parameter`
}

func two(n int, a context.Context, b context.Context) { // want `context.Context is not the first parameter`
}

type server struct{}
