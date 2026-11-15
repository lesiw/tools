// Command vet checks Go packages with the lesiw.io analyzer suite.
//
//	go tool lesiw.io/tools/cmd/vet [packages]
//
// It reports diagnostics to standard error and exits non-zero when there are
// any. The -fix flag applies the suite's suggested fixes.
//
// Each analyzer below carries the name it is selected by, one paragraph on
// what it reports, and an example of the code it reports. The example is the
// fault: the code vet would flag, with the diagnostic's subject visible in the
// code rather than in a comment.
//
// # addrnil
//
// Reports a comparison of an address with nil. Taking an address never yields
// nil, so the comparison has the same answer wherever it is evaluated.
//
// Inspired by staticcheck's SA4022.
//
//	func unset(p *int) bool {
//		return &p == nil
//	}
//
// # afterhttperror
//
// Reports an http.Error call that a later statement in the same block
// follows, where that statement uses the same writer. http.Error writes the
// response and returns, so the handler goes on to write a second one.
//
// Inspired by go-critic's returnAfterHttpError.
//
//	func serve(w http.ResponseWriter) {
//		http.Error(w, "denied", http.StatusForbidden)
//		w.WriteHeader(http.StatusOK)
//	}
//
// # any
//
// Reports the empty interface type, written interface{}, in a package that
// declares Go 1.18 or later. The alias any names the same type.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(v interface{}) {}
//
// # appendassign
//
// Reports an append whose result is assigned to a different slice, where the
// function reads the first argument again. That argument keeps its old length,
// so the later read misses the appended value.
//
// Adapted from go-critic's appendAssign.
//
//	func grow(xs, ys []int) ([]int, []int) {
//		ys = append(xs, 1)
//		return xs, ys
//	}
//
// # appendcombine
//
// Reports consecutive appends to the same slice. Each append may grow the
// slice and copy it; one append with every value grows it once.
//
// Inspired by go-critic's appendCombine.
//
//	func pull() []int {
//		var xs []int
//		xs = append(xs, 1)
//		xs = append(xs, 2)
//		return xs
//	}
//
// # appends
//
// Reports a call to append that passes no values after the slice. The call
// returns the slice unchanged.
//
// Imported from the x/tools vet suite.
//
//	func f(xs []string) []string {
//		return append(xs)
//	}
//
// # argorder
//
// Reports a call whose arguments name the parameters in the other order. Two
// adjacent parameters of one type can be swapped without a compile error, so
// only the names show which value went where.
//
// Inspired by go-critic's argOrder.
//
//	func send(from, to string) {}
//
//	func f(from, to string) {
//		send(to, from)
//	}
//
// # asmdecl
//
// Reports a disagreement between an assembly function and the Go declaration
// it implements: an argument or result read at the wrong size or offset, a
// frame size that differs, a missing Go declaration, or a RET that leaves the
// result unwritten.
//
// Imported from the x/tools vet suite.
//
//	func add(x, y int64)
//
//	TEXT ·add(SB), NOSPLIT, $0-8
//		RET
//
// # assertnil
//
// Reports a nil comparison on the result of a type assertion whose success the
// same if condition already tests. A successful assertion to an interface type
// yields the source's dynamic type, which is never nil.
//
// Inspired by staticcheck's S1020.
//
//	func show(v any) {
//		if s, ok := v.(fmt.Stringer); ok && s != nil {
//			fmt.Println(s)
//		}
//	}
//
// # assign
//
// Reports an assignment that stores an expression into itself, as x = x or
// a[i] = a[i]. The assignment changes nothing.
//
// Imported from the x/tools vet suite.
//
//	func f(xs []int) {
//		xs[0] = xs[0]
//	}
//
// # assignop
//
// Reports an assignment that writes a variable in terms of itself, as
// x = x + y. The compound form x += y states the variable and the operation
// once each.
//
// Inspired by go-critic's assignOp.
//
//	func f(x, y int) {
//		x = x + y
//	}
//
// # atomic
//
// Reports an assignment that stores an atomic call's result back into the
// variable the call read, as x = atomic.AddUint64(&x, 1). The store is a
// second access, so another goroutine's update can be lost.
//
// Imported from the x/tools vet suite.
//
//	func bump(n uint32) {
//		n = atomic.AddUint32(&n, 1)
//	}
//
// # atomicalign
//
// Reports the address of a struct field passed to a 64-bit sync/atomic
// function on a 32-bit platform, where the field is not 64-bit aligned. Such
// a call panics.
//
// Imported from the x/tools vet suite.
//
//	type pair struct {
//		a int32
//		b int64
//	}
//
//	func bump(p *pair) {
//		atomic.AddInt64(&p.b, 1)
//	}
//
// # atomictypes
//
// Reports a call to one of the primitive sync/atomic functions that takes the
// address of a non-exported variable or field, where every use of that
// variable is such a call. The typed atomic values added in Go 1.19 make these
// operations methods, and the variable then admits no non-atomic access.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func bump() {
//		var n int32
//		atomic.AddInt32(&n, 1)
//	}
//
// # badcond
//
// Reports a condition that is always true or always false, as a conjunction
// of bounds that no value satisfies, a disjunction that every value satisfies,
// or a for loop whose init statement makes its condition false. The tool
// cannot tell which condition was meant, so no fix is offered.
//
// Adapted from go-critic's badCond.
//
//	func f(n int) {
//		if n < 0 && n > 10 {
//			return
//		}
//	}
//
// # badsorting
//
// Reports a sort.Slice call whose less function does not order the elements by
// the indexes it was given. A comparison that reads a single element cannot
// order the pair, and one at a constant index reads the same slot every time,
// so the sort leaves the slice as it found it. A fix would guess which pair
// was meant, so none is offered.
//
// Inspired by go-critic's sortSlice, which reads the same less function.
//
//	func f(xs []int) {
//		sort.Slice(xs, func(i, j int) bool { return xs[0] < xs[1] })
//	}
//
// # bookends
//
// Reports bracket layout that the printer would place differently, where a fix
// moves a bracket or a line break and no element changes. The shapes are a
// closer that does not sit at its opener's column, an elided literal whose
// elements start at different columns, a table whose rows do not read down or
// whose row runs past the width, a list with one indented line or a line past
// the width, and a keyed literal on one line past the width.
//
//	func f() []int {
//		return []int{
//			1,
//			2}
//	}
//
// # boolreturn
//
// Reports an if whose branches return the opposite boolean constants, where
// the condition is the value to return. The condition goes into a single
// return, negated when the false branch comes first.
//
// Inspired by staticcheck's S1008.
//
//	func f(ok bool) bool {
//		if ok {
//			return true
//		}
//		return false
//	}
//
// # bools
//
// Reports a chain of && or || that repeats a comparison, as in
// n == 1 && n == 1, or holds two equality tests on one value that fix the
// chain's answer, as in n == 1 && n == 3 or n != 1 || n != 3.
//
// Imported from the x/tools vet suite.
//
//	func f(n int) bool {
//		return n == 1 && n == 3
//	}
//
// # boolset
//
// Reports a map of bool used as a set: every value stored is the literal true.
// A map[T]struct{} states the same intent, and its values take no space. A map
// created in this package is reported, one that arrives from elsewhere, as a
// parameter, a struct field, an exported package variable, or an aliased
// value, is not, and neither is one passed to a call.
//
//	func f(k string) bool {
//		seen := make(map[string]bool)
//		seen[k] = true
//		return seen[k]
//	}
//
// # buildtag
//
// Reports a build constraint that is malformed or misplaced, or a //go:build
// line and a // +build line that disagree about the condition.
//
// Imported from the x/tools vet suite.
//
//	//go:build linux
//	// +build darwin
//
//	package p
//
// # builtinprint
//
// Reports a call to the predeclared print or println. The builtin package
// calls both bootstrapping aids, formats their arguments in an
// implementation-specific way, and does not promise to keep them.
//
// Inspired by revive's use-fmt-print.
//
//	func f(n int) {
//		println("count", n)
//	}
//
// # caseorder
//
// Reports a switch whose literal case values form a contiguous run in neither
// ascending nor descending order. Literal cases with no fallthrough behave the
// same in any order, so the order costs nothing to change.
//
// The name comes from go-critic's caseOrder.
//
//	func f(n int) {
//		switch n {
//		case 3:
//		case 1:
//		case 2:
//		}
//	}
//
// # cgocall
//
// Reports a call into C that passes a value whose type holds a chan, map,
// func, or slice, directly or through a pointer, an array, or a struct field.
// The cgo pointer passing rules forbid such a value from crossing into C.
//
// Imported from the x/tools vet suite.
//
//	func fill(xs []byte) {
//		C.fill(xs)
//	}
//
// # commentfmt
//
// Reports a comment with no space after its slashes. The comment reads as code
// that was commented out, and editors highlight it that way. A doc comment,
// which gofmt spaces itself, is left alone, as is a directive such as
// //go:generate and the prefixes //sys, //sysnb and //nolint.
//
// Inspired by go-critic's commentFormatting.
//
//	func n() int {
//		//return 1
//		return 0
//	}
//
// # composites
//
// Reports an unkeyed composite literal of a struct type declared in another
// package. A field added to that struct later breaks the literal, or shifts
// what each value sets.
//
// Imported from the x/tools vet suite.
//
//	func f() error {
//		return &net.AddrError{"missing port", "addr"}
//	}
//
// # containscut
//
// Reports a Contains test that a Cut over the same operands follows in its
// body. Cut reports whether the separator was present, so the Contains scans
// the same string for an answer that is already available.
//
//	func repoOf(ref string) string {
//		if strings.Contains(ref, "@") {
//			name, _, _ := strings.Cut(ref, "@")
//			return name
//		}
//		return ref
//	}
//
// # copylocks
//
// Reports a value that contains a lock and is copied: passed by value,
// assigned, returned, or read out of a range. Two copies of a lock do not
// exclude each other.
//
// Imported from the x/tools vet suite.
//
//	func f(m sync.Mutex) {
//		m.Lock()
//	}
//
// # ctxfield
//
// Reports a context.Context in a struct field. A context has the deadline and
// the cancellation of one request, so a struct that keeps one cannot serve the
// next request. An embedded context is left alone.
//
// Inspired by golangci-lint's containedctx.
//
//	type server struct {
//		ctx context.Context
//	}
//
// # ctxfirst
//
// Reports a context.Context parameter that is not the first one. A reader
// finds the context in the same place in every signature when it comes first.
//
// Inspired by revive's context-as-argument.
//
//	func handle(n int, ctx context.Context) {}
//
// # ctxguard
//
// Reports an if that opens a function with a test of ctx.Err(), including the
// form that binds the error in its init statement. The first context-aware
// operation in the body observes cancellation itself.
//
//	func work(ctx context.Context) error {
//		if ctx.Err() != nil {
//			return ctx.Err()
//		}
//		return nil
//	}
//
// # ctxkey
//
// Reports a context.WithValue key of a built-in type. A built-in type is the
// same type in every package, so two packages that pick the same value store
// under one key; a defined type keeps them apart.
//
// Adapted from staticcheck's SA1029.
//
//	func withLabel(ctx context.Context) context.Context {
//		return context.WithValue(ctx, "label", 1)
//	}
//
// # ctxname
//
// Reports a context.Context variable, parameter, result, or struct field
// under another name, where renaming it to ctx cannot rebind an identifier. A
// function that holds two contexts under another name is left alone, and so
// is a second context beside a live ctx. So are a package-scope variable, an
// interface method's parameter, and a parameter named parent in a function
// that returns a context.
//
//	func watch(c context.Context) {
//		<-c.Done()
//	}
//
// # cyclo
//
// Reports a function whose cyclomatic complexity is over 15, counted as one
// for the function, one per if, for, range, and && or ||, and one for a
// switch or select, whose clauses cost nothing. A function literal is measured
// separately, and its branches are not counted against the function that
// declares it.
//
//	func f(a, b, c, d, e, g, h, i, j, k, l, m, n, o, p, q bool) bool {
//		return a && b && c && d && e && g && h && i &&
//			j && k && l && m && n && o && p && q
//	}
//
// # deepequalerrors
//
// Reports a call to reflect.DeepEqual with two error values. DeepEqual
// compares the wrapped values field by field, and two errors that describe the
// same failure need not match.
//
// Imported from the x/tools vet suite.
//
//	func same(a, b error) bool {
//		return reflect.DeepEqual(a, b)
//	}
//
// # deepexit
//
// Reports a call to os.Exit, syscall.Exit, or a Fatal function of the log
// package outside main or init. The call ends the program at that point, so
// the caller gets no error to handle.
//
// Inspired by revive's deep-exit.
//
//	func serve(err error) {
//		log.Fatal(err)
//	}
//
// # defers
//
// Reports a defer statement whose argument calls time.Since. The argument is
// evaluated where the defer stands, so the elapsed time is measured at once;
// a func literal that makes the call when the defer runs is the fix.
//
// Imported from the x/tools vet suite.
//
//	func f(start time.Time) {
//		defer measure(time.Since(start))
//	}
//
// # deferunlambda
//
// Reports a deferred func literal whose body is a single call that passes no
// arguments. defer f() delays the call itself and evaluates its arguments at
// the same moment.
//
// Inspired by go-critic's deferUnlambda.
//
//	func f() {
//		defer func() { release() }()
//	}
//
// # deprecated
//
// Reports a doc comment that marks something deprecated without the paragraph
// godoc reads. godoc and the editors that render it warn a caller only for a
// paragraph beginning with Deprecated: .
//
// Inspired by go-critic's deprecatedComment.
//
//	// Deprecated, use New.
//	func Old() {}
//
// # directive
//
// Reports a toolchain directive the file cannot hold: one below the package
// clause, a //go:debug line outside a package main or a test file, or a
// space where the directive takes none.
//
// Imported from the x/tools vet suite.
//
//	//go:debug panicnil=1
//	package p
//
// # docsyntax
//
// Reports Markdown that Go doc comments do not implement: a code span, a
// fenced block, bold, or a link with a URL. Go renders each of them as literal
// punctuation.
//
//	// F returns a `value`.
//	func F() {}
//
// # dupbranch
//
// Reports an if whose two branches hold the same statements. The condition has
// no effect there, so either the branches were left half edited or one of them
// was meant to do something else.
//
// Inspired by go-critic's dupBranchBody.
//
//	func f(ok bool) {
//		if ok {
//			done()
//		} else {
//			done()
//		}
//	}
//
// # dupcase
//
// Reports a case that names an expression an earlier case in the same switch
// named. The second case can never match, and the compiler folds only
// constants.
//
// Inspired by go-critic's dupCase.
//
//	func f(xs []int) int {
//		switch xs[0] {
//		case xs[1]:
//			return 1
//		case xs[1]:
//			return 2
//		}
//		return 0
//	}
//
// # dupimport
//
// Reports one package imported twice under different names. Both names reach
// the same package, so a reader has to check which one a use site means.
//
// Inspired by go-critic's dupImport.
//
//	import (
//		"bytes"
//		printing "bytes"
//	)
//
// # duplarg
//
// Reports a call that passes one identifier as both arguments of a function in
// a fixed set of library functions, where the answer cannot depend on two
// equal values. strings.Contains(s, s) is always true, math.Max(n, n) is n,
// and copy(xs, xs) copies a slice onto itself. A call on any other function is
// left alone.
//
// Inspired by go-critic's dupArg.
//
//	func f(s string) bool {
//		return strings.Contains(s, s)
//	}
//
// # dupsubexpr
//
// Reports a binary operator whose operands are the same value, as
// xs[k] < xs[k]. One of them was meant to name something else. A NaN changes
// the answer of ==, !=, <=, >=, - and /, so an operand that can hold one is
// left alone for those operators, and so is a call whose second read can
// differ.
//
// Adapted from go-critic's dupSubExpr.
//
//	func f(xs []int, k int) bool {
//		return xs[k] < xs[k]
//	}
//
// # earlyreturn
//
// Reports a return whose chain of conditions mixes && with || and does not fit
// one line. Early returns give each way out a guard and leave one answer at
// the end.
//
//	func f(s string) bool {
//		return strings.Contains(s, "alpha") && strings.HasPrefix(s, "a") ||
//			strings.Contains(s, "beta")
//	}
//
// # elseif
//
// Reports an if nested in an else block. Writing it as else if removes a pair
// of braces and one level of indent.
//
// Inspired by go-critic's elseif.
//
//	func f(n int) int {
//		if n == 0 {
//			return 0
//		} else {
//			if n == 1 {
//				return 1
//			}
//		}
//		return -1
//	}
//
// # embedlit
//
// Reports an embedded type named in a composite literal, or a field of it
// assigned just after, where Go 1.27 sets the promoted field in the outer
// literal instead.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	type item struct {
//		base
//	}
//
//	type base struct {
//		kind string
//	}
//
//	var first = item{base: base{kind: "alpha-one"}}
//
// # emptydecl
//
// Reports a var, const, type, or import block that declares nothing. The block
// was emptied and left behind.
//
// Inspired by go-critic's emptyDecl.
//
//	var ()
//
// # emptyfallthrough
//
// Reports a case whose only statement is a fallthrough. Listing both values in
// one case does the same without the jump.
//
// Inspired by go-critic's emptyFallthrough.
//
//	func f(n int) int {
//		switch n {
//		case 0:
//			fallthrough
//		case 1:
//			return 1
//		}
//		return 0
//	}
//
// # emptystring
//
// Reports a comparison of a string's length with zero. len(s) == 0 and
// s == "" are the same test, and the second is the form a reader expects.
//
// Inspired by go-critic's emptyStringTest.
//
//	func f(s string) bool {
//		return len(s) == 0
//	}
//
// # equalfold
//
// Reports a comparison of two values converted to the same case. Each side
// builds a copy, where strings.EqualFold or bytes.EqualFold compares the
// originals without one.
//
// Adapted from go-critic's equalFold.
//
//	func f(a, b string) bool {
//		return strings.ToLower(a) == strings.ToLower(b)
//	}
//
// # errcheck
//
// Reports a call that returns an error whose result is not used, so the
// failure goes unseen. A write to a Buffer or a Builder, a hash's Write, fmt
// output to standard output, a builder, or standard error, a random Read, and
// io.Pipe's CloseWithError are exempt. An error assigned to the blank
// identifier is left alone.
//
//	func save(path string) {
//		os.WriteFile(path, nil, 0o644)
//	}
//
// # errflow
//
// Reports an else block whose if block ends in a return, a break, a continue,
// a goto, or a panic. The else runs only when the block did not take that
// exit, so its code reads at the function's level. An else if chain, and an
// else under an if with an init statement, are left alone.
//
// Inspired by revive's indent-error-flow.
//
//	func f(err error) int {
//		if err != nil {
//			return 0
//		} else {
//			return 1
//		}
//	}
//
// # errfmt
//
// Reports an error built with errors.New, which fmt.Errorf also does, and an
// error formatted with %v or %s where %w would wrap it. Only a literal format
// string and an argument whose static type is error are considered.
//
//	func f(err error) error {
//		return fmt.Errorf("open failed: %v", err)
//	}
//
// # errname
//
// Reports a sentinel error whose name does not begin with Err, and an error
// type whose name does not end in Error. The names are how a reader tells the
// two apart.
//
// Imported from Antonboom's errname.
//
//	var NotFound = errors.New("not found")
//
// # errorlast
//
// Reports a signature whose error result is not the last one. A caller writes
// the error last, as in n, err := lookup(). A result is an error when its type
// is the predeclared error or implements error under a name ending in Error.
//
// Adapted from revive's error-return.
//
//	func lookup() (error, int) {
//		return nil, 0
//	}
//
// # errorsas
//
// Reports a call to errors.As whose target is not a non-nil pointer to a type
// that implements error, and a target of type *error.
//
// Imported from the x/tools vet suite.
//
//	func f(err error) {
//		var target os.PathError
//		errors.As(err, target)
//	}
//
// # errorsastype
//
// Reports an errors.As call on a pointer target declared just above it, where
// errors.AsType gives the same test as a typed value and a bool.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(err error) {
//		var target *os.PathError
//		if errors.As(err, &target) {
//			use(target)
//		}
//	}
//
// # errstrings
//
// Reports an error string that starts with a capital letter or ends with
// punctuation. The message usually follows other text, as in reading config: ,
// so a capital inside it reads as a sentence break the caller did not write.
//
// Inspired by revive's error-strings.
//
//	var errMissing = errors.New("Not found")
//
// # evalorder
//
// Reports a return that reads a value and passes its address to a call in the
// same statement. The spec fixes the order of neither, so the read may see the
// value before or after the call writes it.
//
// Adapted from go-critic's evalOrder.
//
//	func f(n int) (int, int) {
//		return n, bump(&n)
//	}
//
// # filepathjoin
//
// Reports a path concatenated with a literal separator, in a file that imports
// path/filepath. filepath.Join writes the separator of the platform and cleans
// the result.
//
// Adapted from go-critic's preferFilepathJoin.
//
//	import "path/filepath"
//
//	func config(dir, name string) string {
//		return filepath.Dir(dir) + "/" + name
//	}
//
// # findallzero
//
// Reports a FindAll call on a regexp.Regexp whose count is zero. A count of n
// returns at most n matches and a negative count returns every match, so zero
// returns none.
//
// Inspired by staticcheck's SA1010.
//
//	func f(s string) []string {
//		return regexp.MustCompile("a+").FindAllString(s, 0)
//	}
//
// # flagderef
//
// Reports a flag value read from the pointer that flag.Bool and its siblings
// return. Before Parse the pointer holds the default, so a read gets the
// default rather than the flag the user set.
//
// Inspired by go-critic's flagDeref.
//
//	func f() {
//		_ = *flag.String("name", "", "a name")
//	}
//
// # flagname
//
// Reports a flag name that begins with a hyphen or contains an equal sign.
// FlagSet.Var has panicked on such a name since Go 1.17, and a declaration
// that never runs registers a flag no command line can set.
//
// Adapted from go-critic's flagName.
//
//	var level = flag.Int("-level", 0, "")
//
// # forvar
//
// Reports a range loop that copies its loop variable into a fresh one with
// x := x. Since Go 1.22 each iteration has its own variable, so the copy does
// nothing.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(xs []int) {
//		for _, x := range xs {
//			x := x
//			use(x)
//		}
//	}
//
// # framepointer
//
// Reports assembly that writes the frame pointer in a function that has not
// saved it. The runtime walks the stack through that register.
//
// Imported from the x/tools vet suite.
//
//	TEXT ·compare(SB), NOSPLIT, $0
//		MOVQ 8(SP), BP
//		RET
//
// # gofixdirective
//
// Reports a //go:fix inline directive on a declaration that gives the
// toolchain nothing to substitute, as a constant whose value is a literal
// rather than another named constant.
//
// Imported from the x/tools vet suite.
//
//	//go:fix inline
//	const answer = 42
//
// # gospell
//
// Reports an English word spelled as Go does not spell it. Go writes American
// English, and the standard library shows it: context.Canceled has one l. The
// rule reads comments and string literals; an identifier may be a name from
// another package.
//
//	// The behaviour is deliberate.
//	func f() {}
//
// # hostport
//
// Reports an address built with fmt.Sprintf from a host and a port and passed
// to net.Dial. A bare IPv6 host needs brackets in that string, and
// net.JoinHostPort writes them.
//
// Imported from the x/tools vet suite.
//
//	func dial(host string) (net.Conn, error) {
//		return net.Dial("tcp", fmt.Sprintf("%s:%d", host, 80))
//	}
//
// # httpmux
//
// Reports a ServeMux pattern that names a method or a wildcard in a module
// that declares Go 1.21 or earlier. The pattern registers in that release and
// does not match what it was written for.
//
// Imported from the x/tools vet suite.
//
//	func routes(mux *http.ServeMux) {
//		mux.HandleFunc("GET /items", list)
//	}
//
// # httpresponse
//
// Reports a read of an http.Response whose error has not been tested, as a
// deferred Close on a response that may be nil. The error says whether the
// response exists.
//
// Imported from the x/tools vet suite.
//
//	func head(url string) error {
//		resp, err := http.Head(url)
//		defer resp.Body.Close()
//		return err
//	}
//
// # ifaceassert
//
// Reports a type assertion to an interface that the source interface cannot
// implement, because a method of the same name has a different signature. The
// assertion can never hold.
//
// Imported from the x/tools vet suite.
//
//	func f(v interface{ Read() }) {
//		_ = v.(io.Reader)
//	}
//
// # ifelsechain
//
// Reports an if-else chain of four or more branches. A switch puts each
// condition at one indent, where a reader looks for it.
//
// Inspired by go-critic's ifElseChain.
//
//	func f(n int) int {
//		if n == 0 {
//			return 0
//		} else if n == 1 {
//			return 1
//		} else if n == 2 {
//			return 2
//		} else if n == 3 {
//			return 3
//		}
//		return -1
//	}
//
// # importcomment
//
// Reports a package clause with an import comment naming the canonical path.
// The go command reads the comment in GOPATH mode only, so a module ignores
// it.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	package p // import "example.com/p"
//
// # importgroup
//
// Reports an import block that is not grouped standard library, other
// modules, then the packages the local flag names, one blank line between the
// groups and each group sorted by path.
//
//	import (
//		"os"
//
//		"fmt"
//	)
//
// # importshadow
//
// Reports a declaration that shadows an imported package name. A reader who
// misses the declaration reads every later reference as the package.
//
// Inspired by go-critic's importShadow.
//
//	import "path"
//
//	func join(dir, name string) string {
//		return path.Join(dir, name)
//	}
//
//	func clean(path string) string {
//		return path
//	}
//
// # increment
//
// Reports an increment or decrement written as a compound assignment. The spec
// gives i++ and i += 1 the same meaning, and Go code writes i++.
//
// Adapted from revive's increment-decrement.
//
//	func f(i int) int {
//		i += 1
//		return i
//	}
//
// # indexalloc
//
// Reports a byte slice converted to a string only to be searched. The
// conversion copies the bytes, and the bytes package searches the slice in
// place.
//
// Inspired by go-critic's indexAlloc.
//
//	func f(b []byte) int {
//		return strings.Index(string(b), "alpha")
//	}
//
// # initclause
//
// Reports an if or switch init clause whose statement declares nothing.
// Effective Go describes the clause as the place to set up a local variable,
// so a call there reads as something else.
//
// Inspired by go-critic's initClause.
//
//	func f(m map[string]int, k string) bool {
//		if delete(m, k); len(m) == 0 {
//			return true
//		}
//		return false
//	}
//
// # initialisms
//
// Reports a name whose initialism is written in mixed case. Go writes an
// initialism in one case throughout a name: URLPath and urlPath, never
// UrlPath.
//
// Inspired by golint's commonInitialisms.
//
//	type UrlPath struct{}
//
// # ipliteral
//
// Reports an IP address written into source as part of a URL, a host:port, or
// a user@host. An address in a string cannot move to DNS or fail over, and
// loopback and the unspecified address are left alone. A test file is left
// alone too, since a fixture names the place it talks to on purpose.
//
//	func dial() string {
//		return "203.0.113.9:22"
//	}
//
// # lenzero
//
// Reports a comparison of len with zero that is always true or always false.
// len returns a count, and a count is never negative.
//
// Inspired by go-critic's sloppyLen.
//
//	func f(xs []int) bool {
//		return len(xs) < 0
//	}
//
// # linelen
//
// Reports a line wider than the limit, counted in columns with a tab as four.
// An import, a function signature, a generated file, and a line whose width is
// mostly one string literal or URL are exempt.
//
//	func f() {
//		fmt.Println("alpha", "bravo", "charlie", "delta")
//	}
//
// # linewrap
//
// Reports an if, for, or switch header that spans more than one line. The
// opening brace of the body sits on the line the statement begins, and a
// header spread over several lines puts the branch point under a block of
// arguments.
//
//	func f(ctx context.Context, arg int) {
//		if err := call(
//			ctx, arg); err != nil {
//			return
//		}
//	}
//
// # loopclosure
//
// Reports a function literal that reads a loop variable of an enclosing loop
// and is called in a way that outlives the iteration, as a go or defer
// statement does. In a file that declares a Go version before 1.22 every
// iteration shares the variable, so the literal can read a value its own
// iteration never had.
//
// Imported from the x/tools vet suite.
//
//	//go:build go1.21
//
//	package p
//
//	func f(xs []int) {
//		for _, x := range xs {
//			go func() {
//				println(x)
//			}()
//		}
//	}
//
// # lostcancel
//
// Reports a context cancel function that is never called. The context and
// everything derived from it stay live until its parent is canceled.
//
// Imported from the x/tools vet suite.
//
//	func f(ctx context.Context) context.Context {
//		next, _ := context.WithCancel(ctx)
//		return next
//	}
//
// # makezero
//
// Reports a slice made with a non-zero constant length by := and then appended
// to. The append writes past the elements the make left there, and they stay
// in the result.
//
// Inspired by golangci-lint's makezero.
//
//	func f() []int {
//		xs := make([]int, 4)
//		return append(xs, 1)
//	}
//
// # mapfind
//
// Reports a range over a map whose body searches for one key. An index
// expression reaches that entry directly, where the range visits every key.
//
// Adapted from revive's inefficient-map-lookup.
//
//	func f(m map[string]int, want string) (int, bool) {
//		for k, v := range m {
//			if k == want {
//				return v, true
//			}
//		}
//		return 0, false
//	}
//
// # mapkey
//
// Reports a map key that is unhashable. A map keyed by an interface accepts a
// slice, a map, or a function, and the map panics when it hashes one. For a
// conversion to an interface the report names the operand's type, so any(xs)
// names []int.
//
// The name comes from go-critic's mapKey.
//
//	func f(m map[any]int, xs []int) {
//		m[xs] = 1
//	}
//
// # mapsloop
//
// Reports a range over a map or an iter.Seq2 whose body copies one entry into
// another map. maps.Copy writes the map loop as one call, maps.Insert the
// iterator loop, in a package that declares Go 1.23 or later.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(dst, src map[string]int) {
//		for k, v := range src {
//			dst[k] = v
//		}
//	}
//
// # marshalrecv
//
// Reports a write to the receiver of MarshalJSON, MarshalText, MarshalBinary,
// or GobEncode declared with a value receiver. The method works on a copy, so
// the write never reaches the caller.
//
// Inspired by revive's modifies-value-receiver.
//
//	func (i item) MarshalText() ([]byte, error) {
//		i.text = nil
//		return nil, nil
//	}
//
// # methodexprcall
//
// Reports a method expression called with its receiver. The method call names
// the receiver once, where the expression names it again as the argument.
//
// Adapted from go-critic's methodExprCall.
//
//	func f(b *strings.Builder) string {
//		return (*strings.Builder).String(b)
//	}
//
// # minmax
//
// Reports a test that picks one of two values for a variable, which min or
// max writes on one line. A floating-point operand is left alone, where the
// two forms differ at a NaN.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(a, b int) int {
//		x := a
//		if a < b {
//			x = b
//		}
//		return x
//	}
//
// # newderef
//
// Reports a dereference of a fresh allocation. new(T) zeroes a T and the
// dereference reads the zero back, which T{} writes directly.
//
// Inspired by go-critic's newDeref.
//
//	func f() int {
//		return *new(int)
//	}
//
// # newexpr
//
// Reports a function whose body only returns the address of its parameter.
// Go 1.26 gives new the same call, and the fix marks the function for
// inlining.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func ptr(v int) *int {
//		return &v
//	}
//
// # nilchecklen
//
// Reports a nil test joined to a comparison of len that already answers it.
// The length of a nil slice, map, or channel is zero.
//
// Inspired by staticcheck's S1009.
//
//	func f(xs []int) bool {
//		return xs != nil && len(xs) > 0
//	}
//
// # nilfunc
//
// Reports a comparison of a function with nil. A function declared in the
// source is never nil, so the comparison is always false.
//
// Imported from the x/tools vet suite.
//
//	func f() {}
//
//	func g() bool {
//		return f == nil
//	}
//
// # nilness
//
// Reports a nil test whose answer the assignments already fix, as a pointer
// known to be nil or known to be set, and a dereference of a nil pointer.
//
// Imported from the x/tools vet suite.
//
//	func f() {
//		var p *int
//		if p != nil {
//			use(p)
//		}
//	}
//
// # nilreturn
//
// Reports a return of an error the if just found to be nil. The return always
// yields nil, so either the test is inverted or the return should be nil. A
// nil guard on another type is left alone.
//
// Inspired by go-critic's nilValReturn.
//
//	func f(err error) error {
//		if err == nil {
//			return err
//		}
//		return nil
//	}
//
// # nowsub
//
// Reports time.Now().Sub(t) and t.Sub(time.Now()), which time.Since and
// time.Until are shorthand for. The time.Since direction is reported only when
// the argument does no work, because the rewrite moves the clock read past the
// argument.
//
// Adapted from staticcheck's S1012 and S1024.
//
//	func f(start time.Time) time.Duration {
//		return time.Now().Sub(start)
//	}
//
// # octal
//
// Reports an octal literal written with a leading zero. The 0o prefix marks
// the base where a leading zero reads as padding. An argument of time.Date
// keeps its zero, and so does a literal of zero, or one written 0_755.
//
// Inspired by go-critic's octalLiteral.
//
//	const mode = 0755
//
// # offby1
//
// Reports an index at the length of the value it indexes. The last element
// sits at one less, so the index is out of range on every call.
//
// Inspired by go-critic's offBy1.
//
//	func f(xs []int) int {
//		return xs[len(xs)]
//	}
//
// # omitzero
//
// Reports an omitempty tag on a struct-typed field. A struct value never
// encodes as empty, so the tag does nothing; omitzero, added in Go 1.24,
// omits the field when it is zero.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	type label struct {
//		kind string
//	}
//
//	type item struct {
//		label label `json:"label,omitempty"`
//	}
//
// # plusbuild
//
// Reports a // +build line in a file that also holds a //go:build line. The
// older line is obsolete since Go 1.18, and gofmt removes it.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	//go:build linux
//	// +build linux
//
//	package p
//
// # preferdecoderune
//
// Reports a string converted to a rune slice to read its first rune. The
// conversion decodes every rune, where utf8.DecodeRuneInString decodes one.
//
// Adapted from go-critic's preferDecodeRune.
//
//	func f(s string) rune {
//		return []rune(s)[0]
//	}
//
// # preferfprint
//
// Reports a formatted string built with fmt and then written to a writer.
// fmt.Fprintf formats into the writer, so the string in between is never
// built.
//
// Inspired by go-critic's preferFprint.
//
//	func f(b *bytes.Buffer, n int) {
//		b.WriteString(fmt.Sprintf("%d", n))
//	}
//
// # preferstringwriter
//
// Reports a string converted to a byte slice and then written to a writer
// whose type declares WriteString. The conversion copies the string, and
// WriteString takes it as it is.
//
// Adapted from go-critic's preferStringWriter.
//
//	func f(b *bytes.Buffer, s string) {
//		b.Write([]byte(s))
//	}
//
// # preferwritebyte
//
// Reports a one-element byte slice literal written to a writer, and that
// literal converted to a string. WriteByte takes the byte, so the literal is
// never built.
//
// Inspired by go-critic's preferWriteByte.
//
//	func f(b *bytes.Buffer) {
//		b.Write([]byte{'\n'})
//		b.WriteString(string([]byte{'0'}))
//	}
//
// # printf
//
// Reports a format string whose verbs and arguments disagree, and a formatting
// call with the wrong number of arguments. The check covers fmt, log, and a
// wrapper of either.
//
// Imported from the x/tools vet suite.
//
//	func f(n int) string {
//		return fmt.Sprintf("%d", "one")
//	}
//
// # rangeappendall
//
// Reports a range loop that appends each element of a slice one at a time.
// Each append may grow the slice and copy it; one append with the whole slice
// grows it once.
//
// Inspired by go-critic's rangeAppendAll, which reports the opposite loop.
//
//	func merge(dst, src []string) []string {
//		for _, s := range src {
//			dst = append(dst, s)
//		}
//		return dst
//	}
//
// # rangeint
//
// Reports a three-clause for loop that counts to an integer and leaves its
// counter alone in the body. Go 1.22 writes the same loop with a range.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(n int) {
//		for i := 0; i < n; i++ {
//			use(i)
//		}
//	}
//
// # recvname
//
// Reports a method receiver whose name differs from the name another method of
// the type uses, and one named this, self, or me. A reader who sees two names
// has to check whether the second is a different value.
//
// Inspired by revive's receiver-naming.
//
//	func (c *client) get() {}
//
//	func (cl *client) post() {}
//
// # recvtype
//
// Reports a type whose methods mix pointer and value receivers. A value
// receiver works on a copy, and one kind for the whole type settles which
// methods a value has.
//
// Inspired by golangci-lint's recvcheck.
//
//	func (p *pair) swap() {}
//
//	func (p pair) at() {}
//
// # redundantalias
//
// Reports an import alias that repeats the name the import already gets. The
// spec makes the name in the imported package's clause the default.
//
// Inspired by revive's redundant-import-alias.
//
//	import (
//		fmt "fmt"
//	)
//
// # redundantbuildtag
//
// Reports a //go:build line the file name already implies. The go command
// includes a file only where the tags in its name hold.
//
// Adapted from revive's redundant-build-tag.
//
//	// a_linux.go
//	//go:build linux
//
//	package a
//
// # redundantsprint
//
// Reports a fmt.Sprint whose operand has static type string, and a string
// conversion around one. Both return the operand unchanged. A named string
// type is left alone, since Sprint may call its String method.
//
// Adapted from go-critic's redundantSprint.
//
//	func f(s string) string {
//		return fmt.Sprint(s)
//	}
//
// # reflecttypeassert
//
// Reports a two-value type assertion on the result of reflect.Value.Interface.
// reflect.TypeAssert asserts on the value itself, without the interface in
// between.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(v reflect.Value) (string, bool) {
//		return v.Interface().(string)
//	}
//
// # reflecttypefor
//
// Reports a reflect.TypeOf call that names a type. reflect.TypeFor takes that
// type as a type argument and reads the same, from Go 1.22.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f() reflect.Type {
//		return reflect.TypeOf(uint32(0))
//	}
//
// # reflectvaluecompare
//
// Reports a comparison of two reflect.Value values, with == or
// reflect.DeepEqual. Both compare the reflect package's representation in
// place of the values inside.
//
// Imported from the x/tools vet suite.
//
//	func f(a, b reflect.Value) bool {
//		return a == b
//	}
//
// # regexpconst
//
// Reports a regexp pattern that does not compile, where the pattern is a
// constant. MustCompile panics and the other functions return the error on
// every run.
//
// Adapted from staticcheck's SA1000.
//
//	var re = regexp.MustCompile("a(")
//
// # regexpmust
//
// Reports a regexp.Compile of a constant pattern whose error the caller reads.
// The pattern is fixed, so the error is always nil, and MustCompile compiles
// the same pattern.
//
// Adapted from go-critic's regexpMust.
//
//	func must() *regexp.Regexp {
//		re, _ := regexp.Compile("^a+$")
//		return re
//	}
//
// # selfassert
//
// Reports a type assertion from a type to itself. The assertion yields the
// value it was given, so it does nothing. A nil value panics in the assertion,
// so removing it would change behavior, and the rule reports without a fix.
//
// Inspired by go-critic's sloppyTypeAssert.
//
//	func f(v any) any {
//		return v.(any)
//	}
//
// # shellcommand
//
// Reports a call to exec.Command or exec.CommandContext whose program name
// holds whitespace. The package does not run a shell, so the whole string is
// looked up as one file name and the call usually fails. A program path that
// holds a space runs, and the rule reports it too.
//
// Adapted from staticcheck's SA1005.
//
//	func f() *exec.Cmd {
//		return exec.Command("alpha --one")
//	}
//
// # shift
//
// Reports a shift by a count at least as large as the operand's width, which
// moves every bit out of the value.
//
// Imported from the x/tools vet suite.
//
//	func f(n uint8) uint8 {
//		return n << 8
//	}
//
// # sigchanyzer
//
// Reports a signal.Notify call on an unbuffered channel. The signal package
// sends without blocking, so a signal that arrives with no receiver is lost.
//
// Imported from the x/tools vet suite.
//
//	func f(ch chan os.Signal) {
//		signal.Notify(ch, os.Interrupt)
//	}
//
// # singlefield
//
// Reports a struct of one field that the type could carry instead. A pointer,
// interface, embedded, or tagged field is left alone. The field and the type
// fold into a defined type, and for an exported one that changes every
// importer.
//
//	type name struct {
//		text string
//	}
//
// # sliceclear
//
// Reports a loop that writes the zero value to every element of a slice.
// clear does the same in one call, from Go 1.21.
//
// Adapted from go-critic's sliceClear.
//
//	func reset(xs []int) {
//		for i := range xs {
//			xs[i] = 0
//		}
//	}
//
// # slicesbackward
//
// Reports a loop that reads a slice from its last element to its first.
// slices.Backward yields the same index and element, from Go 1.23.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(xs []int) {
//		for i := len(xs) - 1; i >= 0; i-- {
//			use(xs[i])
//		}
//	}
//
// # slicesclip
//
// Reports a full slice expression that clips the capacity to the length.
// slices.Clip writes the same expression, from Go 1.21.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(xs []int) []int {
//		return xs[:len(xs):len(xs)]
//	}
//
// # slicescontains
//
// Reports a loop that searches a slice for one value. slices.Contains and
// slices.ContainsFunc state the search in one call, from Go 1.21.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(xs []int, want int) bool {
//		for _, x := range xs {
//			if x == want {
//				return true
//			}
//		}
//		return false
//	}
//
// # slicessort
//
// Reports a sort.Slice over an ordered basic type that compares the elements
// by index. slices.Sort orders the same slice, from Go 1.21.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(xs []int) {
//		sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
//	}
//
// # slog
//
// Reports a log/slog call whose key-value pairs are wrong: an argument in a
// key position that is neither a string nor a slog.Attr, or a final key with
// no value.
//
// Imported from the x/tools vet suite.
//
//	func f() {
//		slog.Warn("message", 11, 12)
//	}
//
// # sloppyreassign
//
// Reports an assignment in the init clause of an if, switch, or for whose
// variable nothing outside the statement uses. A short declaration gives the
// variable the narrower scope where it is read.
//
// Adapted from go-critic's sloppyReassign.
//
//	func f(name string) error {
//		var err error
//		if err = fetch(name); err != nil {
//			return err
//		}
//		return nil
//	}
//
// # sortmismatch
//
// Reports a sort.Slice, sort.SliceStable, or sort.SliceIsSorted call whose
// less function does not name the slice the call reads. The call orders, or
// tests, the elements by values it reads from somewhere else.
//
// Adapted from go-critic's sortSlice.
//
//	func f(rows []row, keys []string) {
//		sort.Slice(rows, func(i, j int) bool { return keys[i] < keys[j] })
//	}
//
// # sortslice
//
// Reports a sort.Slice call whose first argument is not a slice. The call
// takes an interface value, so the sort panics when it reaches the argument.
//
// Imported from the x/tools vet suite.
//
//	func f(m map[string]int) {
//		sort.Slice(m, func(i, j int) bool { return true })
//	}
//
// # sprintfquotedstring
//
// Reports a format string that writes quotes around a %s by hand. A quote or
// a backslash in the value ends the quoted text early, where %q escapes the
// value and supplies the quotes.
//
// Adapted from go-critic's sprintfQuotedString.
//
//	func f(s string) string {
//		return fmt.Sprintf("\"%s\"", s)
//	}
//
// # stditerators
//
// Reports a loop that counts to a standard library type's count and reads each
// element through its index method. The type offers an iterator over the same
// elements, and a range writes the loop.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(t reflect.Type) {
//		for i := 0; i < t.NumField(); i++ {
//			use(t.Field(i))
//		}
//	}
//
// # stdmethods
//
// Reports a method whose name matches a method of a well-known interface and
// whose signature does not match it. The type then does not satisfy the
// interface it looks like it implements.
//
// Imported from the x/tools vet suite.
//
//	type buffer struct{}
//
//	func (b buffer) WriteTo(w io.Writer) error { return nil }
//
// # stdversion
//
// Reports a reference to a standard library symbol that a later Go release
// than the file's added. The file's version is its module's go directive or
// the //go:build line at its top, and no file counts as older than Go 1.21.
//
// Imported from the x/tools vet suite.
//
//	//go:build go1.21
//
//	package p
//
//	import "maps"
//
//	func keys(m map[string]int) {
//		_ = maps.Keys(m)
//	}
//
// # strcompare
//
// Reports a comparison of a strings.Compare or bytes.Compare result with zero.
// The comparison operators express the same test, and strings.Compare
// documents them as clearer and faster. A bytes.Compare result is reported
// only against == and !=, since its ordering has no shorter form.
//
// Inspired by go-critic's stringsCompare.
//
//	func f(a, b string) bool {
//		return strings.Compare(a, b) == 0
//	}
//
// # strconcat
//
// Reports a strings.Join of two or three strings with a literal separator. A
// concatenation allocates once and needs no slice literal in between.
//
// Inspired by go-critic's stringConcatSimplify.
//
//	func f(a, b string) string {
//		return strings.Join([]string{a, b}, "-")
//	}
//
// # strictvar
//
// Reports a declaration the suite would write another way, such as a zero
// value given a value, a non-zero value under var, adjacent declarations left
// ungrouped, a preamble spread over several blocks, or an immediate call where
// a var and an init function do. The rewrite keeps every value and comment.
//
//	func f() int {
//		x := 0
//		return x
//	}
//
// # stringintconv
//
// Reports a conversion of an integer to a string. The conversion yields the
// UTF-8 encoding of the number as a code point, where the decimal digits are
// what the code usually means. A rune or a byte is left alone, since the
// conversion is the intended form for both.
//
// Imported from the x/tools vet suite.
//
//	func f(n int) string {
//		return string(n)
//	}
//
// # stringsbuilder
//
// Reports string concatenation with +=. strings.Builder accumulates the text
// without copying the string at every step.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(xs []string) string {
//		s := ""
//		for _, x := range xs {
//			s += x
//		}
//		return s
//	}
//
// # stringscut
//
// Reports a search with strings.Index or strings.LastIndex and a slice at the
// result. strings.Cut returns the text either side of the separator and
// whether it was there, from Go 1.18.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(s string) string {
//		if i := strings.Index(s, "@"); i >= 0 {
//			return s[i+1:]
//		}
//		return s
//	}
//
// # stringscutprefix
//
// Reports a HasPrefix test followed by a TrimPrefix of the same prefix.
// strings.CutPrefix returns the remainder and whether the prefix was there,
// from Go 1.20.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(s string) string {
//		if strings.HasPrefix(s, "/") {
//			return strings.TrimPrefix(s, "/")
//		}
//		return s
//	}
//
// # stringsseq
//
// Reports a range over the Split or Fields function of strings or bytes. The
// Seq forms, added in Go 1.24, yield the substrings without allocating a
// slice.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(s string) {
//		for _, field := range strings.Fields(s) {
//			use(field)
//		}
//	}
//
// # stringxbytes
//
// Reports a conversion between a string and a byte slice that the call does
// not need: copy and append take a string, len and an empty test keep the
// count, and Write pairs with WriteString. The conversion copies the bytes.
//
// Inspired by go-critic's stringXbytes and preferStringWriter.
//
//	func f(dst []byte, src string) int {
//		return copy(dst, []byte(src))
//	}
//
// # structtag
//
// Reports a struct field tag that reflect.StructTag.Get cannot read, as one
// that is not a quoted key-value pair, and an unexported field tagged for
// encoding/json or encoding/xml, which those packages skip.
//
// Imported from the x/tools vet suite.
//
//	type item struct {
//		name string `json:"name"`
//	}
//
// # switchtrue
//
// Reports a switch whose tag is the literal true. A switch with no tag
// compares every case with true.
//
// Inspired by go-critic's switchTrue.
//
//	func f(n int) int {
//		switch true {
//		case n > 0:
//			return 1
//		}
//		return 0
//	}
//
// # testcmp
//
// Reports a reflect.DeepEqual call in a test file. DeepEqual compares
// unexported fields and says only that two values differ, where a comparator
// such as go-cmp can say where.
//
//	func TestSame(t *testing.T) {
//		if !reflect.DeepEqual(1, 1) {
//			t.Error("differs")
//		}
//	}
//
// # testhelpers
//
// Reports a test helper that reaches for process-global state where testing
// scopes one to the test: t.Context for context.Background and context.TODO,
// t.Setenv for os.Setenv, t.TempDir for os.MkdirTemp, and t.Chdir for
// os.Chdir.
//
//	func TestWrite(t *testing.T) {
//		os.Setenv("KIND", "alpha")
//	}
//
// # testingcontext
//
// Reports a cancelable context created in a test and canceled by a deferred
// call. t.Context, added in Go 1.24, cancels the context when the test ends.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func TestRun(t *testing.T) {
//		ctx, cancel := context.WithCancel(context.Background())
//		defer cancel()
//		use(ctx)
//	}
//
// # testinggoroutine
//
// Reports a call to Fatal, Fatalf, FailNow, Skip, Skipf, or SkipNow in a
// goroutine a test starts. Those methods stop the goroutine that calls them,
// and a goroutine a test starts is not the one running the test.
//
// Imported from the x/tools vet suite.
//
//	func TestRun(t *testing.T) {
//		go func() {
//			t.Fatal("stopped")
//		}()
//	}
//
// # testlabel
//
// Reports a subtest whose name is a constant string. A subtest is one case in
// a family, and a constant describes one thing.
//
//	func TestEvict(t *testing.T) {
//		t.Run("removes the entry", func(t *testing.T) {})
//	}
//
// # tests
//
// Reports a test function whose name or signature is wrong, and an example
// that documents an identifier the package does not have. The functions
// checked are Test, Benchmark, Fuzz, and Example.
//
// Imported from the x/tools vet suite.
//
//	func TestBad(n int) {}
//
// # tidytypes
//
// Reports a parameter or result that writes a type the name before it already
// gave. One type covers a run of names.
//
//	func f(a int, b int) int { return a + b }
//
// # timeafter
//
// Reports a time.NewTimer used only through a deferred Stop and one receive in
// a select beside its declaration. time.After writes the same wait, and since
// Go 1.23 the collector reclaims the timer.
//
//	func f(ch chan int) error {
//		timer := time.NewTimer(time.Second)
//		defer timer.Stop()
//		select {
//		case <-timer.C:
//			return errTimeout
//		}
//		return nil
//	}
//
// # timeequal
//
// Reports a comparison of two time.Time values with == or !=, including a
// comparison with the zero value. The operator compares the location and the
// monotonic reading too, and Equal and IsZero compare the instant.
//
// Inspired by revive's time-equal.
//
//	func f(a, b time.Time) bool {
//		return a == b
//	}
//
// # timeformat
//
// Reports a format that writes the year, the day, then the month. No common
// date standard runs year-day-month, so 2006-01-02 is usually what was meant.
//
// Imported from the x/tools vet suite.
//
//	func f(t time.Time) string {
//		return t.Format("2006-02-01")
//	}
//
// # timelayout
//
// Reports a time layout that names no reference element, prints the hour of a
// 12-hour clock with no PM or pm, or does not parse back. A layout is the
// reference time, Mon Jan 2 15:04:05 MST 2006, written the way the output
// should read.
//
// Adapted from staticcheck's SA1002.
//
//	func f(t time.Time) string {
//		return t.Format("yyyy-mm-dd")
//	}
//
// # todocomment
//
// Reports a TODO, FIXME, or BUG doc comment with no detail. The note gives the
// next reader nothing to act on.
//
// Inspired by go-critic's todoCommentWithoutDetail.
//
//	// TODO
//	func f() {}
//
// # truncatecmp
//
// Reports a comparison through a conversion to a narrower integer type. The
// conversion keeps the low bits, so the comparison reads a truncated value.
//
// Adapted from go-critic's truncateCmp.
//
//	func f(wide int64, bound int32) bool {
//		return int32(wide) < bound
//	}
//
// # typeassertchain
//
// Reports an if-else chain of type assertions on one value. A type switch
// states the expression once and puts each type in a case.
//
// Inspired by go-critic's typeAssertChain.
//
//	func f(x any) {
//		if v, ok := x.(int); ok {
//			print(v)
//		} else if v, ok := x.(string); ok {
//			print(v)
//		}
//	}
//
// # typedefirst
//
// Reports a type declared after a method on it. A reader who opens the file to
// learn what the type holds meets a method first.
//
// Adapted from go-critic's typeDefFirst.
//
//	func (s *session) touch() {
//		s.count++
//	}
//
//	type session struct {
//		count int
//	}
//
// # typeswitchvar
//
// Reports a type switch whose cases assert the value again. Binding the value
// in the switch header gives each case its typed value once.
//
// Inspired by go-critic's typeSwitchVar.
//
//	func f(x any) int {
//		switch x.(type) {
//		case int:
//			return x.(int)
//		}
//		return 0
//	}
//
// # typeunparen
//
// Reports parentheses around a type in a position gofmt leaves alone, as a var
// or type declaration or a map value. The type reads the same without them.
//
// Inspired by go-critic's typeUnparen.
//
//	var x (int) = 1
//
// # uncondrecursion
//
// Reports a call that runs whenever the function does and calls the function
// again, as an unguarded recursive call does. The stack grows until the
// program crashes.
//
// Adapted from revive's unconditional-recursion.
//
//	func f() {
//		f()
//	}
//
// # underef
//
// Reports a dereference in front of a selector. The spec makes x.f shorthand
// for (*x).f when f is a field, and both panic on a nil pointer.
//
// Adapted from go-critic's underef.
//
//	func kind(p *item) string {
//		return (*p).Kind
//	}
//
// # unlabelstmt
//
// Reports a label that no goto names and that every break or continue reaching
// it would reach without the name. The label adds nothing.
//
// Inspired by go-critic's unlabelStmt.
//
//	func f(xs []int) {
//	outer:
//		for range xs {
//			break outer
//		}
//	}
//
// # unlambda
//
// Reports a function literal whose body only forwards its arguments to
// another function. Naming that function does the same with less code. A
// literal whose func type is wider than the function's is left alone.
//
// Inspired by go-critic's unlambda.
//
//	func f() {
//		use(func(s string) []string { return strings.Fields(s) })
//	}
//
// # unmarshal
//
// Reports a value passed to an unmarshal or decode function that is neither a
// pointer nor an interface. The function writes through the argument, so a
// value it cannot write through stays as it was.
//
// Imported from the x/tools vet suite.
//
//	func f(b []byte) error {
//		var v map[string]int
//		return json.Unmarshal(b, v)
//	}
//
// # unnecessaryblock
//
// Reports a block statement that scopes nothing. A name's scope ends at the
// innermost block that contains it, so a block in a statement list that
// declares nothing adds a scope nothing uses. A clause is already a block, so
// a block that is the whole of a case or select clause is reported even when
// it declares a name.
//
// Adapted from go-critic's unnecessaryBlock.
//
//	func f(n int) {
//		{
//			println(n)
//		}
//	}
//
// # unnecessarydefer
//
// Reports a deferred call that ends the function, where a plain call runs at
// the same moment. The defer adds a keyword and nothing else.
//
// Inspired by go-critic's unnecessaryDefer.
//
//	func f() {
//		defer release()
//	}
//
// # unnecessaryformat
//
// Reports a fmt.Printf, fmt.Sprintf, fmt.Fprintf, or testing Errorf call whose
// format string holds no verb. The Print, Sprint, Fprint, and Error functions
// write the same text.
//
// Inspired by revive's unnecessary-format.
//
//	func f() {
//		fmt.Printf("done\n")
//	}
//
// # unreachable
//
// Reports a statement that execution cannot reach, as one below a return, a
// panic, or an endless loop.
//
// Imported from the x/tools vet suite.
//
//	func f() int {
//		return 1
//		println("never")
//	}
//
// # unsafefuncs
//
// Reports pointer arithmetic written through uintptr conversions, as
// unsafe.Pointer(uintptr(p) + n). unsafe.Add and unsafe.Slice write the same
// arithmetic, and the garbage collector can follow them.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(p unsafe.Pointer, n uintptr) unsafe.Pointer {
//		return unsafe.Pointer(uintptr(p) + n)
//	}
//
// # unsafeptr
//
// Reports a conversion of a uintptr to unsafe.Pointer that leaves a stored
// uintptr holding an address. The garbage collector cannot see that word, and
// a stack copy moves what it points to.
//
// Imported from the x/tools vet suite.
//
//	func f(u uintptr) *int {
//		return (*int)(unsafe.Pointer(u))
//	}
//
// # unslice
//
// Reports a slice expression that takes the whole value, as s[:]. The
// expression returns the value it was given.
//
// Inspired by go-critic's unslice.
//
//	func f(s []int) []int {
//		return s[:]
//	}
//
// # unusedparams
//
// Reports a parameter of a declared function that the body never reads.
// Renaming it to the blank identifier says the position is deliberate, and an
// exported function, a matching interface method, and a function used as a
// value keep their names.
//
// A mirror of gopls' unusedparams, which is internal.
//
//	func unread(a int, b string) string {
//		return b
//	}
//
// # unusedrecv
//
// Reports a method receiver whose name the body never reads. Go allows a
// receiver with no name, and a name the body ignores suggests the method works
// on the value.
//
// Inspired by revive's unused-receiver.
//
//	type item struct{}
//
//	func (i *item) read() int {
//		return 0
//	}
//
// # unusedresult
//
// Reports a call whose result is discarded, for the functions whose result is
// their only effect, as fmt.Errorf's is. The set is a fixed list of standard
// library functions, which the -unusedresult.funcs flag extends.
//
// Imported from the x/tools vet suite.
//
//	func f() {
//		fmt.Errorf("never used")
//	}
//
// # unusedwrite
//
// Reports a write to a field or an element of a value that was copied, as the
// loop variable of a range over a slice of structs. The write reaches the copy
// and nothing reads it.
//
// Imported from the x/tools vet suite.
//
//	func f(rows []item) {
//		for _, row := range rows {
//			row.count = 1
//		}
//	}
//
// # unwrap
//
// Reports a construct written across several lines that fits on one: a
// signature, a call, a binary expression, a header, or a block of one spec.
// The width is linelen's.
//
//	func f() {
//		use(1,
//			2)
//	}
//
// # uselessconcat
//
// Reports string literals joined with + where the joined literal fits on the
// line.
//
//	var msg = "one " + "two"
//
// # uuid
//
// Reports a UUID built by formatting hex digits into the shape of one. A UUID
// has one textual form and one parser, and both live in a library.
//
//	func f(b []byte) string {
//		return fmt.Sprintf("%x-%x-%x-%x-%x",
//			b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
//	}
//
// # valswap
//
// Reports a swap written through a temporary variable. Go evaluates every
// right-hand side before it assigns any, so x, y = y, x needs no temporary.
//
// Inspired by go-critic's valSwap.
//
//	func f(x, y int) (int, int) {
//		tmp := x
//		x = y
//		y = tmp
//		return x, y
//	}
//
// # valuereceiver
//
// Reports a write to a field of a value receiver that nothing reads
// afterwards. The method works on a copy, so the write is lost.
//
// Inspired by revive's modifies-value-receiver.
//
//	func (p pair) set(left int) {
//		p.left = left
//	}
//
// # waitgroup
//
// Reports a sync.WaitGroup.Add call made inside a new goroutine, which can run
// after the Wait that waits for it.
//
// Imported from the x/tools vet suite.
//
//	func f(wg *sync.WaitGroup) {
//		go func() {
//			wg.Add(1)
//			defer wg.Done()
//		}()
//		wg.Wait()
//	}
//
// # waitgroupgo
//
// Reports an Add and the go statement that pairs with it through a deferred
// Done. wg.Go, added in Go 1.25, starts the goroutine and keeps the count.
//
// Imported from golang.org/x/tools' modernize suite.
//
//	func f(wg *sync.WaitGroup) {
//		wg.Add(1)
//		go func() {
//			defer wg.Done()
//			work()
//		}()
//	}
//
// # whitespace
//
// Reports a blank line in the wrong place around declarations: one is needed
// between a multi-line top-level declaration and its neighbors, and before a
// doc comment inside a block or a struct field list.
//
//	func b() {
//		println(1)
//	}
//	func c() {}
//
// # wrapperfunc
//
// Reports an unexported function or method with no doc comment whose body only
// forwards its parameters to another function of the same signature. A wrapper
// another package can call, or one the doc comment explains, is left alone. A
// second name for the same behavior is one more name to choose at every call.
//
// The name comes from go-critic's wrapperFunc.
//
//	func outer(x int) int {
//		return inner(x)
//	}
//
// # yoda
//
// Reports an == or != comparison with a literal or nil on the left. Go code
// reads left to right as a sentence about the variable, and a mistyped
// assignment does not compile in Go.
//
// Inspired by go-critic's yodaStyleExpr.
//
//	func f(err error) bool {
//		return nil == err
//	}
//
// # zerorepeat
//
// Reports a run of zero bytes built with bytes.Repeat. make allocates the same
// result and leaves it zeroed.
//
// Inspired by go-critic's zeroByteRepeat.
//
//	func f(n int) []byte {
//		return bytes.Repeat([]byte{0}, n)
//	}
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/suite"
	"lesiw.io/tools/multichecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(suite.Analyzers()...) }
