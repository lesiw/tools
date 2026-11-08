package a

import (
	"context"
	"time"
)

func use(...any) {}

func param(rctx context.Context) { // want `context.Context should be named ctx, not rctx`
	use(rctx)
}

func derived(ctx context.Context) {
	newCtx, cancel := context.WithTimeout(ctx, time.Second) // want `context.Context should be named ctx, not newCtx`
	defer cancel()
	use(newCtx)
}

func outerLive(ctx context.Context) {
	rctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	use(rctx)
	use(ctx.Err())
}

func twoParams(a, b context.Context) {
	use(a, b)
}

func nestedShadow(hctx context.Context) {
	if err := hctx.Err(); err != nil {
		ctx := context.Background()
		use(ctx)
	}
	use(hctx)
}

func WithLabel(parent context.Context) context.Context {
	return parent
}

func sibling(ctx, cancelCtx context.Context) {
	use(ctx, cancelCtx)
}

func litSibling() {
	merge := func(ctx, cancelCtx context.Context) {
		use(ctx, cancelCtx)
	}
	use(merge)
}

func closure(ctx context.Context) {
	f := func(cctx context.Context) { // want `context.Context should be named ctx, not cctx`
		use(cctx)
	}
	f(ctx)
}

func litCapture(ctx context.Context) {
	f := func(cctx context.Context) {
		use(ctx, cctx)
	}
	use(f)
}

func localStruct() {
	type s struct {
		c context.Context // want `context.Context should be named ctx, not c`
	}
	use(s{})
}

type I interface {
	M(c context.Context)
}

type H func(c context.Context)

func blank(_ context.Context) {}

var bg context.Context

func nonDeriving(parent context.Context) { // want `context.Context should be named ctx, not parent`
	use(parent)
}

func later() {
	rctx := context.Background()
	use(rctx)
	ctx := context.Background()
	use(ctx)
}

func joint(octx context.Context) { // want `context.Context should be named ctx, not octx`
	f := func(ictx context.Context) { // want `context.Context should be named ctx, not ictx`
		use(octx, ictx)
	}
	use(f)
}

func litTwoParams() {
	merge := func(a, b context.Context) {
		use(a, b)
	}
	use(merge)
}

type C = context.Context

func aliased(c C) {
	use(c)
}

func earlierVar() {
	ctx := context.Background()
	use(ctx)
	rctx := context.Background()
	use(rctx)
}

func earlierParam(ctx context.Context) {
	use(ctx)
	rctx := context.Background()
	use(rctx)
}

type carrier struct {
	ctx context.Context
}

func selectorField(c context.Context, s carrier) {
	use(c, s.ctx)
}

type request struct {
	rctx context.Context // want `context.Context should be named ctx, not rctx`
	n    int
}

type claimed struct {
	ctx   context.Context
	inner context.Context
}

func splitCtx(ctx context.Context) (context.Context, context.Context) {
	return ctx, ctx
}

func lhsClaim() {
	ctx := context.Background()
	use(ctx)
	rctx, ctx := splitCtx(ctx)
	use(rctx, ctx)
}

func rootCtx() context.Context {
	parent := context.Background() // want `context.Context should be named ctx, not parent`
	return parent
}

func fromParent(parent context.Context) error { // want `context.Context should be named ctx, not parent`
	use(parent)
	return nil
}

func supply() (rctx context.Context) { // want `context.Context should be named ctx, not rctx`
	rctx = context.Background()
	return rctx
}

func allRedeclared() {
	ctx := context.Background()
	cancel := func() {}
	use(ctx, cancel)
	rctx, cancel := context.WithCancel(ctx)
	use(rctx, cancel)
}
