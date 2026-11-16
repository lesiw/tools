// Package encodeoverlap provides an analysis.Analyzer that reports an encoder
// called with a destination and a source that start at the same index of one
// backing array.
//
// encoding/base64's Encoding.Encode "encodes src using the encoding enc"
// (https://pkg.go.dev/encoding/base64#Encoding.Encode). It writes
// Encoding.EncodedLen(len(src)) bytes to dst, and the documentation does not
// say what an overlap does. The encoder reads three source bytes and writes
// four destination bytes, so a destination that starts where the source starts
// can overwrite source bytes before they are read. The other encoders write to
// the destination before they read the next source bytes too; their EncodedLen
// and MaxEncodedLen methods and functions return an output length or an upper
// bound
// (https://pkg.go.dev/encoding/base32#Encoding.EncodedLen,
// https://pkg.go.dev/encoding/hex#EncodedLen,
// https://pkg.go.dev/encoding/ascii85#MaxEncodedLen).
//
// The rule reports a call to one of those encoders when both arguments are
// slices of one backing array that start at the same index. An assignment to a
// local name, a conversion that keeps the backing array, and a branch that
// takes the same value on every path carry the value to the call. Two reads of
// the same variable or field in one block count as the same slice when no
// call, write, or send lies between them. A nil constant, a different array,
// a different lower bound, and a zero-length source are left alone. The README
// catalogs every check, exemption, and limit.
//
// Report only: a repair copies the source into a separate buffer before the
// call, and the rule cannot tell whether the added allocation is acceptable.
//
// The rule is inspired by staticcheck's SA1031.
package encodeoverlap

import (
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:     "encodeoverlap",
	Doc:      "reports an encoder called with a destination and a source at the same index",
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	var (
		info  = pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
		seen  = make(map[*ssa.Function]struct{})
		visit func(*ssa.Function)
	)
	visit = func(fn *ssa.Function) {
		if fn == nil {
			return
		}
		if _, ok := seen[fn]; ok {
			return
		}
		seen[fn] = struct{}{}
		checkFunction(pass, fn)
		for _, anon := range fn.AnonFuncs {
			visit(anon)
		}
	}
	for _, fn := range info.SrcFuncs {
		visit(fn)
	}
	if init, ok := info.Pkg.Members["init"].(*ssa.Function); ok {
		visit(init)
	}
	return nil, nil
}

// checkFunction reports each overlapping encoder call in fn.
func checkFunction(pass *analysis.Pass, fn *ssa.Function) {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			var call *ssa.CallCommon
			switch instr := instr.(type) {
			case *ssa.Call:
				call = &instr.Call
			case *ssa.Defer:
				call = &instr.Call
			case *ssa.Go:
				call = &instr.Call
			default:
				continue
			}
			checkCall(pass, call)
		}
	}
}

// checkCall reports call when it is an encoder call whose destination and
// source share memory.
func checkCall(pass *analysis.Pass, call *ssa.CallCommon) {
	if call.IsInvoke() {
		return
	}
	callee := call.StaticCallee()
	if callee == nil {
		return
	}
	fn, ok := callee.Object().(*types.Func)
	if !ok {
		return
	}
	dst, src, ok := arguments(fn)
	if !ok {
		return
	}
	if dst == 1 && len(call.Args) == src {
		report(pass, call.Pos(), call.Args[0], call.Args[1])
		return
	}
	if len(call.Args) != src+1 {
		return
	}
	report(pass, call.Pos(), call.Args[dst], call.Args[src])
}

// report reports a call at pos when dst and src share memory.
func report(pass *analysis.Pass, pos token.Pos, dst, src ssa.Value) {
	if !overlap(dst, src) {
		return
	}
	pass.Reportf(pos, "dst and src share memory")
}

// arguments returns the positions of the destination and source in a call to
// fn, including the receiver for a method call.
func arguments(fn *types.Func) (dst, src int, ok bool) {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return 0, 0, false
	}
	if sig.Recv() != nil {
		if !encodingMethod(fn, sig) {
			return 0, 0, false
		}
		return 1, 2, true
	}
	pkg := fn.Pkg()
	if pkg == nil || fn.Name() != "Encode" {
		return 0, 0, false
	}
	switch pkg.Path() {
	case "encoding/ascii85", "encoding/hex":
		return 0, 1, true
	}
	return 0, 0, false
}

// encodingMethod reports whether fn is the Encode method of an Encoding in
// encoding/base32 or encoding/base64.
func encodingMethod(fn *types.Func, sig *types.Signature) bool {
	if fn.Name() != "Encode" {
		return false
	}
	ptr, ok := types.Unalias(sig.Recv().Type()).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() == nil || obj.Name() != "Encoding" {
		return false
	}
	switch obj.Pkg().Path() {
	case "encoding/base32", "encoding/base64":
		return true
	}
	return false
}

// overlap reports whether dst and src are the same slice, or slices that
// start at the same index of one backing array.
func overlap(dst, src ssa.Value) bool {
	var ok bool
	dst, ok = flatten(dst)
	if !ok {
		return false
	}
	src, ok = flatten(src)
	if !ok {
		return false
	}
	if nilSlice(dst) || nilSlice(src) {
		return false
	}
	if zeroLength(src) {
		return false
	}
	if isSliceType(dst) && isSliceType(src) && sameValue(dst, src) {
		return true
	}
	dstBase, dstLow, ok := sliceOf(dst)
	if !ok {
		return false
	}
	srcBase, srcLow, ok := sliceOf(src)
	if !ok {
		return false
	}
	dstBase, ok = flatten(dstBase)
	if !ok {
		return false
	}
	srcBase, ok = flatten(srcBase)
	if !ok {
		return false
	}
	if !sameValue(dstBase, srcBase) && !sameAddress(dstBase, srcBase) {
		return false
	}
	return sameLow(dstLow, srcLow)
}

// flatten returns the one value a chain of conversions and branches reaches,
// or nil when the chain reaches more than one value.
func flatten(v ssa.Value) (ssa.Value, bool) {
	var (
		seen   = make(map[ssa.Value]struct{})
		out    ssa.Value
		failed bool
		walk   func(ssa.Value)
		record = func(v ssa.Value) {
			if out == nil {
				out = v
			} else if out != v {
				failed = true
			}
		}
	)
	walk = func(v ssa.Value) {
		if v == nil || failed {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		switch v := v.(type) {
		case *ssa.Phi:
			for _, edge := range v.Edges {
				walk(edge)
			}
		case *ssa.ChangeType:
			walk(v.X)
		case *ssa.Convert:
			if isSliceType(v.X) && isSliceType(v) {
				walk(v.X)
				return
			}
			record(v)
		default:
			record(v)
		}
	}
	walk(v)
	if failed {
		return nil, false
	}
	return out, true
}

// sameValue reports whether a and b hold the same value, including two reads
// of one variable or field with nothing writing between them.
func sameValue(a, b ssa.Value) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a == b {
		return true
	}
	if fa, ok := a.(*ssa.Field); ok {
		fb, ok := b.(*ssa.Field)
		return ok && fa.Field == fb.Field && sameValue(fa.X, fb.X)
	}
	var (
		ua, aok = a.(*ssa.UnOp)
		ub, bok = b.(*ssa.UnOp)
	)
	if !aok || !bok || ua.Op != token.MUL || ub.Op != token.MUL {
		return false
	}
	return sameAddress(ua.X, ub.X) && quietBetween(ua, ub)
}

// quietBetween reports whether two loads share a block and no call, write, or
// send lies between them.
func quietBetween(a, b *ssa.UnOp) bool {
	if a.Block() == nil || a.Block() != b.Block() {
		return false
	}
	var (
		instrs = a.Block().Instrs
		ai     = instructionIndex(instrs, a)
		bi     = instructionIndex(instrs, b)
	)
	if ai < 0 || bi < 0 {
		return false
	}
	if ai > bi {
		ai, bi = bi, ai
	}
	for _, instr := range instrs[ai+1 : bi] {
		switch instr.(type) {
		case *ssa.Store, *ssa.Call, *ssa.Go, *ssa.Defer,
			*ssa.MapUpdate, *ssa.Send:
			return false
		}
	}
	return true
}

// instructionIndex returns the position of want in instrs, or -1.
func instructionIndex(instrs []ssa.Instruction, want ssa.Instruction) int {
	for i, instr := range instrs {
		if instr == want {
			return i
		}
	}
	return -1
}

// sameAddress reports whether a and b name the same variable or field.
func sameAddress(a, b ssa.Value) bool {
	var aok, bok bool
	a, aok = flatten(a)
	b, bok = flatten(b)
	if !aok || !bok {
		return false
	}
	if a == nil || b == nil {
		return a == b
	}
	if a == b {
		return true
	}
	switch a := a.(type) {
	case *ssa.FieldAddr:
		other, ok := b.(*ssa.FieldAddr)
		return ok && a.Field == other.Field && sameAddress(a.X, other.X)
	case *ssa.IndexAddr:
		other, ok := b.(*ssa.IndexAddr)
		return ok && sameLow(a.Index, other.Index) && sameAddress(a.X, other.X)
	case *ssa.UnOp:
		other, ok := b.(*ssa.UnOp)
		return ok && a.Op == token.MUL && other.Op == token.MUL &&
			sameAddress(a.X, other.X) && quietBetween(a, other)
	}
	return false
}

// nilSlice reports whether v is the nil constant.
func nilSlice(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	return ok && c.Value == nil
}

// zeroLength reports whether v is a slice whose length is known to be zero.
func zeroLength(v ssa.Value) bool {
	switch v := v.(type) {
	case *ssa.Slice:
		if v.High != nil {
			return sameLow(v.Low, v.High)
		}
		return zeroArray(v.X)
	case *ssa.MakeSlice:
		return zero(v.Len)
	}
	return false
}

// zeroArray reports whether v is a pointer to a zero-length array.
func zeroArray(v ssa.Value) bool {
	if v == nil || v.Type() == nil {
		return false
	}
	t := v.Type()
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		t = ptr.Elem()
	}
	array, ok := t.Underlying().(*types.Array)
	return ok && array.Len() == 0
}

// sliceOf returns the value a slice is taken from and its lower bound. A
// slice value with no slice operation is its own base.
func sliceOf(v ssa.Value) (base, low ssa.Value, ok bool) {
	if v == nil {
		return nil, nil, false
	}
	if slice, ok := v.(*ssa.Slice); ok {
		return slice.X, slice.Low, true
	}
	if isSliceType(v) {
		return v, nil, true
	}
	return nil, nil, false
}

// isSliceType reports whether v is a slice.
func isSliceType(v ssa.Value) bool {
	if v == nil || v.Type() == nil {
		return false
	}
	_, ok := v.Type().Underlying().(*types.Slice)
	return ok
}

// sameLow reports whether two slice lower bounds name the same value. An
// omitted bound is zero.
func sameLow(a, b ssa.Value) bool {
	if a == nil {
		return b == nil || zero(b)
	}
	if b == nil {
		return zero(a)
	}
	if sameValue(a, b) {
		return true
	}
	var (
		ac, aok = a.(*ssa.Const)
		bc, bok = b.(*ssa.Const)
	)
	if !aok || !bok || ac.Value == nil || bc.Value == nil {
		return false
	}
	return constant.Compare(ac.Value, token.EQL, bc.Value)
}

// zero reports whether v is the constant zero.
func zero(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	return ok && c.Value != nil && constant.Sign(c.Value) == 0
}
