// Package marshalfunc provides an analysis.Analyzer that reports a call to
// encoding/json that marshals a value whose type contains a channel or a func.
//
// The Marshal documentation says: "Channel, complex, and function values
// cannot be encoded in JSON. Attempting to encode such a value causes Marshal
// to return an UnsupportedTypeError." This rule covers channels and functions.
// Marshal returns the error at run time when it encodes a channel or func.
//
// The result depends on the static type alone. A nil pointer, a nil or empty
// slice, and a nil or empty map with a supported key type encode without an
// error and are reported too, as is a field omitted by omitempty or omitzero.
// A value type parameter is reported when an exact term in its constraint
// contains a channel or func. A zero-length array is not reported, since it
// has no element to encode.
//
// A value whose type implements encoding/json.Marshaler or
// encoding.TextMarshaler is exempt, because the method supplies the encoding.
// A map key is exempt only through encoding.TextMarshaler. An exact key term
// that contains a channel or func is reported at x[key]; a tilde term is left
// alone. The value is checked unless every key type is unsupported and none
// contains a channel or func.
//
// Report only: the repair is to change the type or the data, and neither
// follows from the call.
//
// The rule is inspired by staticcheck's SA1026, narrowed to encoding/json and
// to channels and functions.
package marshalfunc

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "marshalfunc",
	Doc:  "reports a json marshal of a value containing a channel or func",
	Run:  run,
}

// visit records a type and whether json could take its address. A pointer
// receiver's marshal method is reachable only when it could.
type visit struct {
	typ     types.Type
	canAddr bool
}

// finding records the kind of value json cannot encode and its path in the
// type.
type finding struct {
	kind string
	path string
}

// jsonField is a field encoding/json encodes, with its path and whether json
// could take its address.
type jsonField struct {
	name    string
	tag     bool
	index   []int
	typ     types.Type
	canAddr bool
	path    string
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			index, ok := marshaledArg(pass, call)
			if !ok {
				return true
			}
			arg := call.Args[index]
			typ := pass.TypesInfo.TypeOf(arg)
			if typ == nil {
				return true
			}
			f, ok := contains(typ, false, make(map[visit]bool))
			if !ok {
				return true
			}
			message := "marshaling a value whose type contains a " + f.kind
			if f.path != "" {
				message += " via x" + f.path
			}
			pass.Reportf(arg.Pos(), "%s", message)
			return true
		})
	}
	return nil, nil
}

// marshaledArg returns the index of the argument encoding/json marshals.
func marshaledArg(pass *analysis.Pass, call *ast.CallExpr) (int, bool) {
	var (
		sel *ast.SelectorExpr
		fn  *types.Func
	)
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		sel = fun
		fn, _ = pass.TypesInfo.Uses[fun.Sel].(*types.Func)
	case *ast.Ident:
		fn, _ = pass.TypesInfo.Uses[fun].(*types.Func)
	}
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "encoding/json" {
		return 0, false
	}
	switch fn.Name() {
	case "Marshal", "MarshalIndent":
		if len(call.Args) < 1 {
			return 0, false
		}
		return 0, fn.Signature().Recv() == nil
	case "Encode":
		if len(call.Args) < 1 {
			return 0, false
		}
		if fn.Signature().Recv() == nil {
			return 0, false
		}
		if sel != nil && methodExpr(pass, sel) {
			return 1, len(call.Args) > 1
		}
		return 0, true
	}
	return 0, false
}

// methodExpr reports whether sel is a method expression, whose receiver is the
// first argument.
func methodExpr(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	selection, ok := pass.TypesInfo.Selections[sel]
	return ok && selection.Kind() == types.MethodExpr
}

// contains returns the first channel or function in t, and its path. A marshal
// method on t, or on its address when canAddr, prevents json from encoding
// fields.
func contains(t types.Type, canAddr bool, seen map[visit]bool) (finding, bool) {
	if t == nil {
		return finding{}, false
	}
	if param, ok := t.(*types.TypeParam); ok {
		return typeParamContains(param, canAddr)
	}
	if hasMarshaler(t, canAddr) {
		return finding{}, false
	}
	v := visit{typ: t, canAddr: canAddr}
	if seen[v] {
		return finding{}, false
	}
	seen[v] = true

	switch t := t.Underlying().(type) {
	case *types.Chan:
		return finding{kind: "channel"}, true
	case *types.Signature:
		return finding{kind: "func"}, true
	case *types.Struct:
		for _, field := range fields(t, canAddr) {
			f, ok := contains(field.typ, field.canAddr, seen)
			if ok {
				f.path = field.path + f.path
				return f, true
			}
		}
	case *types.Pointer:
		f, ok := contains(t.Elem(), true, seen)
		if ok {
			return f, true
		}
	case *types.Slice:
		f, ok := contains(t.Elem(), true, seen)
		if ok {
			f.path = "[0]" + f.path
			return f, true
		}
	case *types.Array:
		if t.Len() == 0 {
			return finding{}, false
		}
		f, ok := contains(t.Elem(), canAddr, seen)
		if ok {
			f.path = "[0]" + f.path
			return f, true
		}
	case *types.Map:
		f, ok := mapContains(t, seen)
		if ok {
			return f, true
		}
	}
	return finding{}, false
}

// mapContains returns a channel or function that json reaches in a map.
func mapContains(t *types.Map, seen map[visit]bool) (finding, bool) {
	supported, unsupported := keySupport(t.Key())
	if !supported {
		f, ok := keyContains(t.Key(), make(map[visit]bool))
		if ok {
			f.path = "[key]"
			return f, true
		}
		if unsupported {
			return finding{}, false
		}
	}
	f, ok := contains(t.Elem(), false, seen)
	if ok {
		f.path = "[value]" + f.path
		return f, true
	}
	return finding{}, false
}

// typeParamContains reports a channel or function in the parameter's type set.
func typeParamContains(t *types.TypeParam, canAddr bool) (finding, bool) {
	if kind, ok := constraintKind(t.Constraint(), canAddr); ok {
		return finding{kind: kind}, true
	}
	return constraintContains(
		t.Constraint(), canAddr,
		nil, make(map[visit]bool),
	)
}

// constraintContains returns a channel or function in an exact type term of a
// constraint. A tilde term is left alone, because a named type in its set may
// implement a marshal method.
func constraintContains(c types.Type, canAddr bool, filter *types.Interface, seen map[visit]bool) (finding, bool) {
	if c == nil || hasMarshaler(c, canAddr) {
		return finding{}, false
	}
	if union, ok := c.(*types.Union); ok {
		for term := range union.Terms() {
			if term.Tilde() || !implementsFilter(term.Type(), filter) {
				continue
			}
			if f, ok := contains(term.Type(), canAddr, seen); ok {
				return f, true
			}
		}
		return finding{}, false
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		if !implementsFilter(c, filter) {
			return finding{}, false
		}
		return contains(c, canAddr, seen)
	}
	if filter == nil {
		filter = iface
	}
	for etyp := range iface.EmbeddedTypes() {
		if f, ok := constraintContains(etyp, canAddr, filter, seen); ok {
			return f, true
		}
	}
	return finding{}, false
}

// implementsFilter reports whether t satisfies the methods of filter.
func implementsFilter(t types.Type, filter *types.Interface) bool {
	return filter == nil || types.Satisfies(t, filter)
}

// keySupport reports whether json can encode every type in t as a map key,
// and whether it can encode none of them.
func keySupport(t types.Type) (supported, unsupported bool) {
	if t == nil {
		return false, false
	}
	if param, ok := t.(*types.TypeParam); ok {
		return constraintKeySupport(param.Constraint(), nil)
	}
	if keyTermSupported(t) {
		return true, false
	}
	return false, true
}

// keyContains returns a channel or function in an unsupported map key type.
func keyContains(t types.Type, seen map[visit]bool) (finding, bool) {
	if t == nil {
		return finding{}, false
	}
	if param, ok := t.(*types.TypeParam); ok {
		return constraintKeyContains(param.Constraint(), nil, seen)
	}
	v := visit{typ: t}
	if seen[v] {
		return finding{}, false
	}
	seen[v] = true

	switch t := t.Underlying().(type) {
	case *types.Chan:
		return finding{kind: "channel"}, true
	case *types.Signature:
		return finding{kind: "func"}, true
	case *types.Struct:
		for field := range t.Fields() {
			if f, ok := keyContains(field.Type(), seen); ok {
				return f, true
			}
		}
	case *types.Pointer:
		return keyContains(t.Elem(), seen)
	case *types.Slice:
		return keyContains(t.Elem(), seen)
	case *types.Array:
		if t.Len() == 0 {
			return finding{}, false
		}
		return keyContains(t.Elem(), seen)
	case *types.Map:
		if f, ok := keyContains(t.Key(), seen); ok {
			return f, true
		}
		return keyContains(t.Elem(), seen)
	}
	return finding{}, false
}

// constraintKind returns the kind of channel or function that every type in
// the constraint's type set has, when at least one of those types has no
// marshal method.
func constraintKind(c types.Type, canAddr bool) (string, bool) {
	kind, unsafe, terms, ok := constraintSet(c, canAddr, nil)
	if !ok || !terms || !unsafe {
		return "", false
	}
	return kind, true
}

// constraintHasMarshaler reports whether the constraint requires a marshal
// method of every type in its set.
func constraintHasMarshaler(c types.Type) bool {
	iface, ok := c.Underlying().(*types.Interface)
	return ok && implementsAny(iface)
}

// constraintSet describes the type set of a constraint. terms reports whether
// the set is narrowed by a type term, and unsafe reports whether some term has
// no marshal method.
func constraintSet(c types.Type, canAddr bool, filter *types.Interface) (kind string, unsafe, terms, ok bool) {
	if c == nil {
		return "", false, false, false
	}
	if constraintHasMarshaler(c) {
		return "", false, false, false
	}
	if union, ok := c.(*types.Union); ok {
		return constraintUnion(union, canAddr, filter)
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		return constraintType(c, canAddr, filter)
	}
	if filter == nil {
		filter = iface
	}
	for etyp := range iface.EmbeddedTypes() {
		k, u, t, ok := constraintSet(etyp, canAddr, filter)
		if !ok {
			return "", false, false, false
		}
		if !t {
			continue
		}
		kind = mergeKind(kind, k)
		unsafe = unsafe || u
		terms = true
	}
	return kind, unsafe, terms, true
}

// constraintUnion describes a union term of a constraint.
func constraintUnion(union *types.Union, canAddr bool, filter *types.Interface) (kind string, unsafe, terms, ok bool) {
	for term := range union.Terms() {
		if !implementsFilter(term.Type(), filter) {
			continue
		}
		if term.Tilde() {
			return "", false, false, false
		}
		k, ok := typeKind(term.Type())
		if !ok {
			return "", false, false, false
		}
		kind = mergeKind(kind, k)
		terms = true
		if !hasMarshaler(term.Type(), canAddr) {
			unsafe = true
		}
	}
	return kind, unsafe, terms, true
}

// constraintType describes a non-interface type term of a constraint.
func constraintType(c types.Type, canAddr bool, filter *types.Interface) (kind string, unsafe, terms, ok bool) {
	if !implementsFilter(c, filter) {
		return "", false, false, false
	}
	k, ok := typeKind(c)
	if !ok {
		return "", false, false, false
	}
	return k, !hasMarshaler(c, canAddr), true, true
}

// constraintKeySupport reports whether every type in the constraint's type set
// can be a map key, and whether none of them can.
func constraintKeySupport(c types.Type, filter *types.Interface) (supported, unsupported bool) {
	if c == nil {
		return false, false
	}
	if types.Implements(c, textMarshaler) {
		return true, false
	}
	if union, ok := c.(*types.Union); ok {
		return constraintKeyUnion(union, filter)
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		return constraintKeyType(c, filter)
	}
	if filter == nil {
		filter = iface
	}
	var someSupported, someUnsupported, terms bool
	for etyp := range iface.EmbeddedTypes() {
		s, u := constraintKeySupport(etyp, filter)
		if !s && !u {
			continue
		}
		if !terms {
			someSupported, someUnsupported = true, true
			terms = true
		}
		someSupported = someSupported && s
		someUnsupported = someUnsupported && u
	}
	return someSupported, someUnsupported
}

// constraintKeyUnion reports key support for a union term of a constraint.
func constraintKeyUnion(union *types.Union, filter *types.Interface) (supported, unsupported bool) {
	allSupported, allUnsupported, terms := true, true, false
	for term := range union.Terms() {
		if !implementsFilter(term.Type(), filter) {
			continue
		}
		if term.Tilde() {
			terms = true
			allUnsupported = false
			if !keyBasic(term.Type()) {
				allSupported = false
			}
			continue
		}
		terms = true
		if keyTermSupported(term.Type()) {
			allUnsupported = false
		} else {
			allSupported = false
		}
	}
	if !terms {
		return false, false
	}
	if allSupported {
		return true, false
	}
	if allUnsupported {
		return false, true
	}
	return false, false
}

// constraintKeyType reports key support for a non-interface type term.
func constraintKeyType(c types.Type, filter *types.Interface) (supported, unsupported bool) {
	if !implementsFilter(c, filter) {
		return false, false
	}
	s := keyTermSupported(c)
	return s, !s
}

// constraintKeyContains returns a channel or function in an unsupported map
// key type chosen by a type parameter.
func constraintKeyContains(c types.Type, filter *types.Interface, seen map[visit]bool) (finding, bool) {
	if c == nil {
		return finding{}, false
	}
	if union, ok := c.(*types.Union); ok {
		for term := range union.Terms() {
			if term.Tilde() || !implementsFilter(term.Type(), filter) {
				continue
			}
			if f, ok := keyContains(term.Type(), seen); ok {
				return f, true
			}
		}
		return finding{}, false
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		if !implementsFilter(c, filter) {
			return finding{}, false
		}
		return keyContains(c, seen)
	}
	if filter == nil {
		filter = iface
	}
	for etyp := range iface.EmbeddedTypes() {
		if f, ok := constraintKeyContains(etyp, filter, seen); ok {
			return f, true
		}
	}
	return finding{}, false
}

// keyTermSupported reports whether json accepts t as a map key.
func keyTermSupported(t types.Type) bool {
	return keyBasic(t) || types.Implements(t, textMarshaler)
}

// keyBasic reports whether t is a string or integer type, which json accepts
// as a map key.
func keyBasic(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	info := basic.Info()
	return info&types.IsString != 0 || info&types.IsInteger != 0
}

// typeKind returns the channel or function kind of t.
func typeKind(t types.Type) (string, bool) {
	switch t.Underlying().(type) {
	case *types.Chan:
		return "channel", true
	case *types.Signature:
		return "func", true
	}
	return "", false
}

// mergeKind combines the kinds of two parts of a type set.
func mergeKind(a, b string) string {
	if a == "" {
		return b
	}
	if a == b {
		return a
	}
	return "channel or func"
}

// fields returns the struct fields json encodes after applying its embedding
// and tag rules.
func fields(t types.Type, canAddr bool) []jsonField {
	var (
		current = []jsonField{{typ: t, canAddr: canAddr}}
		count   map[types.Type]int
		visited = make(map[types.Type]struct{})
		found   []jsonField
	)
	for len(current) > 0 {
		var c collector
		c.nextCount = make(map[types.Type]int)
		for _, f := range current {
			if _, ok := visited[f.typ]; ok {
				continue
			}
			visited[f.typ] = struct{}{}
			st, ok := f.typ.Underlying().(*types.Struct)
			if !ok {
				continue
			}
			for i := 0; i < st.NumFields(); i++ {
				c.add(st, f, i, count[f.typ] > 1)
			}
		}
		found = append(found, c.found...)
		current = c.next
		count = c.nextCount
	}
	sort.Slice(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.name != b.name {
			return a.name < b.name
		}
		if len(a.index) != len(b.index) {
			return len(a.index) < len(b.index)
		}
		if a.tag != b.tag {
			return a.tag
		}
		return lessIndex(a.index, b.index)
	})
	out := found[:0]
	for i := 0; i < len(found); {
		j := i + 1
		for j < len(found) && found[j].name == found[i].name {
			j++
		}
		if j-i == 1 {
			out = append(out, found[i])
		} else if f, ok := dominant(found[i:j]); ok {
			out = append(out, f)
		}
		i = j
	}
	return out
}

// collector gathers the fields and embedded structs one level yields.
type collector struct {
	found     []jsonField
	next      []jsonField
	nextCount map[types.Type]int
}

// add records field i of st, or queues its struct type for the next level.
func (c *collector) add(st *types.Struct, parent jsonField, i int, duplicate bool) {
	var (
		sf                   = st.Field(i)
		ft                   = sf.Type()
		embedded, viaPointer = ft, false
	)
	if ptr, ok := ft.Underlying().(*types.Pointer); ok {
		embedded, viaPointer = ptr.Elem(), true
	}
	if sf.Anonymous() {
		if !sf.Exported() && !isStruct(embedded) {
			return
		}
	} else if !sf.Exported() {
		return
	}
	tag := reflect.StructTag(st.Tag(i)).Get("json")
	if tag == "-" {
		return
	}
	name := tagName(tag)
	index := append(append([]int(nil), parent.index...), i)
	path := parent.path + "." + sf.Name()
	if name != "" || !sf.Anonymous() || !isStruct(embedded) {
		tagged := name != ""
		if name == "" {
			name = sf.Name()
		}
		c.found = append(c.found, jsonField{
			name:    name,
			tag:     tagged,
			index:   index,
			typ:     ft,
			canAddr: parent.canAddr,
			path:    path,
		})
		if duplicate {
			c.found = append(c.found, c.found[len(c.found)-1])
		}
		return
	}
	canAddr := parent.canAddr || viaPointer
	c.nextCount[embedded]++
	if c.nextCount[embedded] == 1 {
		c.next = append(c.next, jsonField{
			typ:     embedded,
			canAddr: canAddr,
			path:    path,
		})
	}
}

// tagName returns the name in a json tag, or the empty string when the tag
// supplies none.
func tagName(tag string) string {
	name := tag
	if before, _, ok := strings.Cut(tag, ","); ok {
		name = before
	}
	if !validTag(name) {
		return ""
	}
	return name
}

// dominant returns the single field that wins a name shared by several, or
// false when no field does.
func dominant(fields []jsonField) (jsonField, bool) {
	if len(fields) > 1 && tied(fields[0], fields[1]) {
		return jsonField{}, false
	}
	return fields[0], true
}

// tied reports whether two fields share a depth and tag presence, which leaves
// neither dominant.
func tied(a, b jsonField) bool {
	return len(a.index) == len(b.index) && a.tag == b.tag
}

// lessIndex orders field index sequences lexicographically.
func lessIndex(a, b []int) bool {
	for i, ai := range a {
		if i >= len(b) {
			return false
		}
		if ai != b[i] {
			return ai < b[i]
		}
	}
	return len(a) < len(b)
}

// validTag reports whether s is a valid field name for encoding/json.
func validTag(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", r):
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			return false
		}
	}
	return true
}

func isStruct(t types.Type) bool {
	_, ok := t.Underlying().(*types.Struct)
	return ok
}

var (
	jsonMarshaler = marshaler("MarshalJSON")
	textMarshaler = marshaler("MarshalText")
)

func marshaler(name string) *types.Interface {
	var (
		bytes  = types.NewSlice(types.Typ[types.Uint8])
		err    = types.Universe.Lookup("error").Type()
		method = types.NewFunc(token.NoPos, nil, name, types.NewSignatureType(
			nil, nil, nil, nil,
			types.NewTuple(
				types.NewVar(token.NoPos, nil, "", bytes),
				types.NewVar(token.NoPos, nil, "", err),
			),
			false,
		))
	)
	return types.NewInterfaceType([]*types.Func{method}, nil).Complete()
}

// hasMarshaler reports whether t has a marshal method that supplies the
// encoding. A pointer receiver's method applies only to an addressable value.
func hasMarshaler(t types.Type, canAddr bool) bool {
	if implementsAny(t) {
		return true
	}
	if _, ok := t.Underlying().(*types.Pointer); ok || !canAddr {
		return false
	}
	return implementsAny(types.NewPointer(t))
}

func implementsAny(t types.Type) bool {
	return types.Implements(t, jsonMarshaler) ||
		types.Implements(t, textMarshaler)
}
