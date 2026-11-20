# cmd/vet

Command vet checks Go packages with the lesiw.io analyzer suite.

```
go tool lesiw.io/tools/cmd/vet [packages]
```

It reports diagnostics to standard error and exits non-zero when there are any. The -fix flag applies the suite's suggested fixes.

Each analyzer below carries the name it is selected by, one paragraph on what it reports, and an example of the code it reports. The example is the fault: the code vet would flag, with the diagnostic's subject visible in the code rather than in a comment.

## addrderef

Reports the address of a pointer indirection. The spec says \*x denotes the variable x points to, so &\*x is x when the two types are identical, but &\*x panics on a nil x.

Inspired by staticcheck's SA4001.

```
func f(p *int) *int {
    return &*p
}
```

## addrnil

Reports a comparison of an address with nil. Taking an address never yields nil, so the comparison has the same answer wherever it is evaluated.

Inspired by staticcheck's SA4022.

```
func unset(p *int) bool {
    return &p == nil
}
```

## afterhttperror

Reports an http.Error call that a later statement in the same block follows, where that statement uses the same writer. http.Error writes the response and returns, so the handler goes on to write a second one.

Inspired by go-critic's returnAfterHttpError.

```
func serve(w http.ResponseWriter) {
    http.Error(w, "denied", http.StatusForbidden)
    w.WriteHeader(http.StatusOK)
}
```

## any

Reports the empty interface type, written interface{}, in a package that declares Go 1.18 or later. The alias any names the same type.

Imported from golang.org/x/tools' modernize suite.

```
func f(v interface{}) {}
```

## appendassign

Reports an append whose result is assigned to a different slice, where the function reads the first argument again. That argument keeps its old length, so the later read misses the appended value.

Adapted from go-critic's appendAssign.

```
func grow(xs, ys []int) ([]int, []int) {
    ys = append(xs, 1)
    return xs, ys
}
```

## appendcombine

Reports consecutive appends to the same slice. Each append may grow the slice and copy it; one append with every value grows it once.

Inspired by go-critic's appendCombine.

```
func pull() []int {
    var xs []int
    xs = append(xs, 1)
    xs = append(xs, 2)
    return xs
}
```

## appendloop

Reports a range loop that appends each element of a slice by index, one at a time. append(dst, src...) appends every element in one call, and may grow the slice once instead of once per element.

Adapted from staticcheck's S1011; rangeappendall covers the value form.

```
func merge(dst, src []string) []string {
    for i := range src {
        dst = append(dst, src[i])
    }
    return dst
}
```

## appends

Reports a call to append that passes no values after the slice. The call returns the slice unchanged.

Imported from the x/tools vet suite.

```
func f(xs []string) []string {
    return append(xs)
}
```

## appendself

Reports an append of one value whose result is assigned to a different slice, where the function reads the destination again and does not read the first argument again. The destination's previous elements are absent from the result, so the append does not extend the destination.

Adapted from staticcheck's SA4021, which reports an append that passes no values.

```
func grow(dst, src []int, v int) []int {
    dst = append(src, v)
    return dst
}
```

## appendunused

Reports an append to a slice identifier whose result is assigned to the blank identifier, as \_ = append(xs, v). The returned slice has the new length, and xs keeps its old length.

Adapted from staticcheck's SA4010.

```
func f(xs []int) {
    _ = append(xs, 1)
}
```

## argorder

Reports a call whose arguments name the parameters in the other order. Two adjacent parameters of one type can be swapped without a compile error, so only the names show which value went where.

Inspired by go-critic's argOrder.

```
func send(from, to string) {}

func f(from, to string) {
    send(to, from)
}
```

## argoverwritten

Reports a function parameter whose passed value is never used because the body assigns to it.

Adapted from staticcheck's SA4009.

```
func reset(count int) int {
    count = 0
    return count
}
```

## asmdecl

Reports a disagreement between an assembly function and the Go declaration it implements: an argument or result read at the wrong size or offset, a frame size that differs, a missing Go declaration, or a RET that leaves the result unwritten.

Imported from the x/tools vet suite.

```
func add(x, y int64)

TEXT ·add(SB), NOSPLIT, $0-8
    RET
```

## assertelse

Reports a type assertion whose value is read in the branch that runs when the assertion fails. A failed assertion yields the zero value of the asserted type, so a read there gets that value.

Adapted from staticcheck's SA9008.

```
func f(x any) {
    if v, ok := x.(int); ok {
        print(v)
    } else {
        print(v)
    }
}
```

## assertnil

Reports a nil comparison on the result of a type assertion whose success the same if condition already tests. A successful assertion to an interface type yields the source's dynamic type, which is never nil.

Inspired by staticcheck's S1020.

```
func show(v any) {
    if s, ok := v.(fmt.Stringer); ok && s != nil {
        fmt.Println(s)
    }
}
```

## assign

Reports an assignment that stores an expression into itself, as x = x or a\[i] = a\[i]. The assignment changes nothing.

Imported from the x/tools vet suite.

```
func f(xs []int) {
    xs[0] = xs[0]
}
```

## assignop

Reports an assignment that writes a variable in terms of itself, as x = x + y. The compound form x += y states the variable and the operation once each.

Inspired by go-critic's assignOp.

```
func f(x, y int) {
    x = x + y
}
```

## atoinarrow

Reports a conversion of a strconv.Atoi result to int16, or to int32 where int is 64 bits. Atoi is ParseInt with a base of 10 and a bit size of 0, converted to int, so its result can lie outside int16, or outside int32 where int is 64 bits.

Adapted from gosec's G109.

```
func f(s string) int16 {
    n, _ := strconv.Atoi(s)
    return int16(n)
}
```

## atomic

Reports an assignment that stores an atomic call's result back into the variable the call read, as x = atomic.AddUint64(&x, 1). The store is a second access, so another goroutine's update can be lost.

Imported from the x/tools vet suite.

```
func bump(n uint32) {
    n = atomic.AddUint32(&n, 1)
}
```

## atomicalign

Reports the address of a struct field passed to a 64-bit sync/atomic function on a 32-bit platform, where the field is not 64-bit aligned. Such a call panics.

Imported from the x/tools vet suite.

```
type pair struct {
    a int32
    b int64
}

func bump(p *pair) {
    atomic.AddInt64(&p.b, 1)
}
```

## atomictypes

Reports a call to one of the primitive sync/atomic functions that takes the address of a non-exported variable or field, where every use of that variable is such a call. The typed atomic values added in Go 1.19 make these operations methods, and the variable then admits no non-atomic access.

Imported from golang.org/x/tools' modernize suite.

```
func bump() {
    var n int32
    atomic.AddInt32(&n, 1)
}
```

## badcall

Reports a Replace call whose count is zero, an EqualFold call on two ASCII constants of different lengths, or a write of nil to a hash. Folding maps one byte to one byte in ASCII, so different lengths cannot match.

The Replace check is adapted from go-critic's badCall.

```
func f(s string, h hash.Hash) {
    _ = strings.Replace(s, "a", "b", 0)
    _ = strings.EqualFold("alpha", "alphas")
    h.Write(nil)
}
```

## badcond

Reports a condition that is always true or always false, as a conjunction of bounds that no value satisfies, a disjunction that every value satisfies, or a for loop whose init statement makes its condition false. The tool cannot tell which condition was meant, so no fix is offered.

Adapted from go-critic's badCond.

```
func f(n int) {
    if n < 0 && n > 10 {
        return
    }
}
```

## badlock

Reports a deferred call that does not release the lock taken in the statement before it. A deferred release that names the other method is a run-time error, and a deferred call that takes a lock deadlocks or leaves a read lock that nothing releases.

Adapted from go-critic's badLock.

```
func f(mu *sync.RWMutex) {
    mu.Lock()
    defer mu.RUnlock()
}
```

## badsorting

Reports a sort.Slice call whose less function does not order the elements by the indexes it was given. A comparison that reads a single element cannot order the pair, and one at a constant index reads the same slot every time, so the sort leaves the slice as it found it. A fix would guess which pair was meant, so none is offered.

Inspired by go-critic's sortSlice, which reads the same less function.

```
func f(xs []int) {
    sort.Slice(xs, func(i, j int) bool { return xs[0] < xs[1] })
}
```

## badsyncfunc

Reports a sync.Once.Do call whose function reads a variable that a loop enclosing the call declares, where the loop reuses the Once for every iteration, so only one iteration's function runs.

Inspired by go-critic's badSyncOnceFunc, which reports a sync.OnceFunc value that is discarded or called where it is built.

```
func f(ready *sync.Once, names []string) {
    for _, name := range names {
        ready.Do(func() { use(name) })
    }
}
```

## benchassign

Reports an assignment to the N field of a testing.B. The testing package sets N and adjusts it until a benchmark is long enough to time reliably; assigning to N replaces that count.

Inspired by staticcheck's SA3001.

```
func benchmark(b *testing.B) {
    b.N = 1
}
```

## bidicontrol

Reports a bidirectional control character in a source file. These characters control the direction of the text around them, so the displayed order can differ from the order the compiler parses.

Inspired by gosec's G116.

```
func f() string {
    return "alpha‎ bravo" //ignore:bidicontrol
}
```

## blankassign

Reports a statement that assigns the value of a name or a basic literal to the blank identifier when the assignment has no effect. A name is reported when the code reads it again with no write in between; a basic literal or predeclared constant is reported without another use. A numeric or boolean variable is reported only inside a function, and a numeric or boolean named constant wherever it is declared, each with another read; a variable of any other type can hold a reference or be written through another name for the same storage.

Inspired by staticcheck's S1005.

```
func f() {
    _ = 0
}
```

## blankimports

Reports a blank import in a non-main, non-test file with no comment naming its path. A blank import triggers the imported package's initialization without making a declaration from that package available in the file. Test files, unsafe, and embed with //go:embed are left alone.

Adapted from revive's blank-imports.

```
package a

import _ "net/http/pprof"
```

## blankrange

Reports a range clause with an unnecessary blank identifier.

Inspired by revive's range.

```
func f(xs []int) {
    for _ = range xs {
    }
}
```

## bookends

Reports bracket layout that the printer would place differently, where a fix moves a bracket or a line break and no element changes. The shapes are a closer that does not sit at its opener's column, an elided literal whose opening brace shares a line with its first element, a table whose rows do not read down or whose row runs past the width, a list with one indented line or a line past the width, and a keyed literal on one line past the width.

```
func f() []int {
    return []int{
        1,
        2}
}
```

## boolexprsimplify

Reports a boolean expression with a redundant negation or comparison: a negation of a negation, or a comparison of a bool with true using == or with false using !=. The operand is written in place of the expression.

Inspired by go-critic's boolExprSimplify and staticcheck's S1002.

```
func f(ok bool) bool {
    return ok != false
}
```

## boolliteral

Reports a comparison with a boolean literal, as in ok == true or ok != false. The comparison is replaced by its operand, with a ! where the comparison inverts the operand.

Adapted from revive's bool-literal-in-expr.

```
func f(ok bool) bool {
    return ok == true
}
```

## boolreturn

Reports an if whose branches return the opposite boolean constants, where the condition is the value to return. The condition goes into a single return, negated when the false branch comes first.

Inspired by staticcheck's S1008.

```
func f(ok bool) bool {
    if ok {
        return true
    }
    return false
}
```

## bools

Reports a chain of && or || that repeats a comparison, as in n == 1 && n == 1, or holds two equality tests on one value that fix the chain's answer, as in n == 1 && n == 3 or n != 1 || n != 3.

Imported from the x/tools vet suite.

```
func f(n int) bool {
    return n == 1 && n == 3
}
```

## boolset

Reports a map of bool used as a set: every value stored is the literal true. A map\[T]struct{} states the same intent, and its values take no space. A map created in this package is reported, one that arrives from elsewhere, as a parameter, a struct field, an exported package variable, or an aliased value, is not, and neither is one passed to a call.

```
func f(k string) bool {
    seen := make(map[string]bool)
    seen[k] = true
    return seen[k]
}
```

## breakinselect

Reports a break at the end of a select case in a loop when it leaves only the select. The break leaves the select, and execution continues after the select. A break meant to leave the loop does not leave the loop.

Adapted from staticcheck's SA4011.

```
func f(ch <-chan int) {
    for {
        select {
        case <-ch:
            break
        }
    }
}
```

## bufferbytes

Reports string(buf.Bytes()) and \[]byte(buf.String()). String returns the unread contents as a string; Bytes returns them as a slice that aliases the buffer.

Inspired by staticcheck's S1030.

```
func f(buf *bytes.Buffer) string {
    return string(buf.Bytes())
}
```

## buildtag

Reports a build constraint that is malformed or misplaced, or a //go:build line and a // +build line that disagree about the condition.

Imported from the x/tools vet suite.

```
//go:build linux
// +build darwin

package p
```

## builtinprint

Reports a call to the predeclared print or println. The builtin package calls both bootstrapping aids, formats their arguments in an implementation-specific way, and does not promise to keep them.

Inspired by revive's use-fmt-print.

```
func f(n int) {
    println("count", n)
}
```

## builtinshadow

Reports a declaration that shadows a predeclared identifier. The declared name denotes that declaration in its scope.

Adapted from revive's redefines-builtin-id.

```
var len = 3
```

## caseorder

Reports a switch whose literal case values form a contiguous run in neither ascending nor descending order. Literal cases with no fallthrough behave the same in any order, so the order costs nothing to change.

The name comes from go-critic's caseOrder.

```
func f(n int) {
    switch n {
    case 3:
    case 1:
    case 2:
    }
}
```

## cgocall

Reports a call into C that passes a value whose type holds a chan, map, func, or slice, directly or through a pointer, an array, or a struct field. The cgo pointer passing rules forbid such a value from crossing into C.

Imported from the x/tools vet suite.

```
func fill(xs []byte) {
    C.fill(xs)
}
```

## commentedoutcode

Reports a block of commented-out code. The compiler treats a comment as whitespace, so code in one is not type-checked or formatted. A comment on one line is treated as a note.

Inspired by go-critic's commentedOutCode.

```
func load(names []string) {
    // println(names)
    // loadAll(names)
}
```

## commentfmt

Reports a comment with no space after its slashes. A comment is text after two slashes and a space, and a line with no space reads either as a directive or as code that was commented out. A doc comment, which gofmt spaces itself, is left alone, as is a directive such as //go:generate and the prefixes //sys, //sysnb and //nolint.

Inspired by go-critic's commentFormatting.

```
func n() error {
    return nil //success
}
```

## composites

Reports an unkeyed composite literal of a struct type declared in another package. A field added to that struct later breaks the literal, or shifts what each value sets.

Imported from the x/tools vet suite.

```
func f() error {
    return &net.AddrError{"missing port", "addr"}
}
```

## containscut

Reports a Contains test that a Cut over the same operands follows in its body. Cut reports whether the separator was present, so the Contains scans the same string for an answer that is already available.

```
func repoOf(ref string) string {
    if strings.Contains(ref, "@") {
        name, _, _ := strings.Cut(ref, "@")
        return name
    }
    return ref
}
```

## copylocks

Reports a value that contains a lock and is copied: passed by value, assigned, returned, or read out of a range. Two copies of a lock do not exclude each other.

Imported from the x/tools vet suite.

```
func f(m sync.Mutex) {
    m.Lock()
}
```

## copyto

Reports a loop that copies one slice into another element by element. The builtin copy writes the elements in one call, at most the destination's length, and is defined for overlapping slices.

Adapted from staticcheck's S1001.

```
func f(dst, src []int) {
    for i, v := range src {
        dst[i] = v
    }
}
```

## ctxfield

Reports a context.Context in a struct field. A context has the deadline and the cancellation of one request, so a struct that keeps one cannot serve the next request. An embedded context is left alone.

Inspired by golangci-lint's containedctx.

```
type server struct {
    ctx context.Context
}
```

## ctxfirst

Reports a context.Context parameter that is not the first one. A reader finds the context in the same place in every signature when it comes first.

Inspired by revive's context-as-argument.

```
func handle(n int, ctx context.Context) {}
```

## ctxguard

Reports an if that opens a function with a test of ctx.Err(), including the form that binds the error in its init statement. The first context-aware operation in the body observes cancellation itself.

```
func work(ctx context.Context) error {
    if ctx.Err() != nil {
        return ctx.Err()
    }
    return nil
}
```

## ctxkey

Reports a context.WithValue key of a built-in type. A built-in type is the same type in every package, so two packages that pick the same value store under one key; a defined type keeps them apart.

Adapted from staticcheck's SA1029.

```
func withLabel(ctx context.Context) context.Context {
    return context.WithValue(ctx, "label", 1)
}
```

## ctxname

Reports a context.Context variable, parameter, result, or struct field under another name, where renaming it to ctx cannot rebind an identifier. A function that holds two contexts under another name is left alone, and so is a second context beside a live ctx. So are a package-scope variable, an interface method's parameter, and a parameter named parent in a function that returns a context.

```
func watch(c context.Context) {
    <-c.Done()
}
```

## ctxvaluetype

Reports a context.WithValue call whose value argument is the nil literal. If the parent lacks the key, Value still returns nil; if the parent holds a non-nil value under it, Value returns nil instead.

```
func store(ctx context.Context, key any) context.Context {
    return context.WithValue(ctx, key, nil)
}
```

## cyclo

Reports a function whose cyclomatic complexity is over 15, counted as one for the function, one per if, for, range, and && or ||, and one for a switch or select, whose clauses cost nothing. A function literal is measured separately, and its branches are not counted against the function that declares it.

```
func f(a, b, c, d, e, g, h, i, j, k, l, m, n, o, p, q bool) bool {
    return a && b && c && d && e && g && h && i &&
        j && k && l && m && n && o && p && q
}
```

## deepequalerrors

Reports a call to reflect.DeepEqual with two error values. DeepEqual compares the wrapped values field by field, and two errors that describe the same failure need not match.

Imported from the x/tools vet suite.

```
func same(a, b error) bool {
    return reflect.DeepEqual(a, b)
}
```

## deepequalptr

Reports a reflect.DeepEqual call on a type with an Equal method and a pointer field, or on two pointers. The Equal arm reports a call on such a type, where DeepEqual compares the field's pointee and never calls the method. The identity arm reports a call on two pointer values, where DeepEqual can return true for distinct pointers to equal values.

Inspired by staticcheck's SA1021, which reports a net.IP comparison by its bytes.

```
type item struct {
    meta *int
}

func (i item) Equal(other item) bool {
    return i.meta == other.meta
}

func same(left, right item) bool {
    return reflect.DeepEqual(left, right)
}
```

For two pointer values:

```
func samePointer(left, right *int) bool {
    return reflect.DeepEqual(left, right)
}
```

## deepexit

Reports a call to os.Exit, syscall.Exit, or a Fatal function of the log package outside main or init. The call ends the program at that point, so the caller gets no error to handle.

Inspired by revive's deep-exit.

```
func serve(err error) {
    log.Fatal(err)
}
```

## defaultcaseorder

Reports a default clause in the middle of a switch, a type switch, or a select. A reader looks for the default at the top of the clause list or at the end.

Adapted from go-critic's defaultCaseOrder.

```
func f(n int) {
    switch n {
    case 1:
    default:
    case 2:
    }
}
```

## deferbeforeerr

Reports a deferred call made before the error result is read. The spec evaluates a defer's arguments where the defer stands, so the value is taken before the error that says whether it exists is checked.

Adapted from staticcheck's SA5001.

```
func f() error {
    h, err := open()
    defer h.Close()
    if err != nil {
        return err
    }
    return nil
}
```

## deferchain

Reports a deferred call that returns a function. The function value and arguments are evaluated when the defer statement executes, and the call runs when the surrounding function returns, so the returned function is discarded.

Adapted from staticcheck's SA9010.

```
func chain(cleanup func() func()) {
    defer cleanup()
}
```

## defercloseunchecked

Reports a deferred Close on a file or buffered writer, or a deferred Flush, whose error is discarded. A Write can return while its data is still buffered, and the later Close or Flush writes it and can fail. A Close on an os.File is reported when the file came from a write-opening call, is os.Stdout or os.Stderr, or when its origin is unknown and the function writes through it.

Adapted from gosec's G307, which reports a deferred method that returns an error.

```
func save(w *bufio.Writer) {
    defer w.Flush()
}
```

## deferinfinite

Reports a deferred call inside a for statement that cannot end, whether it has no condition or a condition that is a constant true. A deferred call runs when the function returns, so the number of deferred calls grows by one on each pass.

Adapted from staticcheck's SA5003.

```
func f() {
    for {
        defer release()
    }
}
```

## deferinloop

Reports a defer inside a loop body, where the deferred call runs when the function returns and not when the iteration ends. A loop that runs many times holds every deferred call open at once.

Inspired by go-critic's deferInLoop.

```
func f(paths []string) {
    for _, p := range paths {
        f, err := os.Open(p)
        if err != nil {
            continue
        }
        defer f.Close()
    }
}
```

## defermisuse

Reports a recover call that does not run directly in a deferred function, and a return with results in a deferred literal. The recover builtin stops a panic only when the deferred function calls it directly, and a return from a deferred literal discards its results.

Adapted from revive's defer; defermisuse covers its recover, immediate-recover, and return parts.

```
func f() {
    recover()
}

func g() {
    f()
}
```

## defers

Reports a defer statement whose argument calls time.Since. The argument is evaluated where the defer stands, so the elapsed time is measured at once; a func literal that makes the call when the defer runs is the fix.

Imported from the x/tools vet suite.

```
func f(start time.Time) {
    defer measure(time.Since(start))
}
```

## deferunlambda

Reports a deferred func literal whose body is a single call that passes no arguments. defer f() delays the call itself and evaluates its arguments at the same moment.

Inspired by go-critic's deferUnlambda.

```
func f() {
    defer func() { release() }()
}
```

## deferunlock

Reports a deferred Unlock without Lock, or a deferred RUnlock without RLock, in the same function. The release runs when the function returns, and a sync.Mutex or sync.RWMutex release without a lock is a run-time error.

Adapted from staticcheck's SA5001 family.

```
func release() {
    var mu sync.Mutex
    defer mu.Unlock()
}
```

## deleteguard

Reports an existence check around a delete of the same key. delete does nothing when the key is absent, so the check has no effect.

Adapted from staticcheck's S1033.

```
func f(rows map[string]int, name string) {
    if _, found := rows[name]; found {
        delete(rows, name)
    }
}
```

## deprecated

Reports a use of an identifier or imported package whose doc comment carries a Deprecated: paragraph.

A mirror of gopls' deprecated, which is internal.

```
func f() {
    legacy.Old()
}
```

## deprecatedform

Reports a doc comment that marks something deprecated without the paragraph godoc reads. godoc and the editors that render it warn a caller only for a paragraph beginning with Deprecated: .

Inspired by go-critic's deprecatedComment.

```
// Deprecated, use New.
func Old() {}
```

## deviatingelse

Reports an if/else whose else block does not fall through to the statement after the if. Inverting the condition removes a level of indentation from the then block. The fix writes a comparison with the opposite operator, and negates a compound condition whole.

Inspired by revive's early-return.

```
func show(n int) {
    if n == 0 {
        print(n)
    } else {
        return
    }
}
```

## directive

Reports a toolchain directive the file cannot hold: one below the package clause, a //go:debug line outside a package main or a test file, or a space where the directive takes none.

Imported from the x/tools vet suite.

```
//go:debug panicnil=1
package p
```

## discardedpure

Reports a call whose only effect is its result, written as a statement. The result is discarded, so the statement does nothing.

Adapted from staticcheck's SA4017.

```
func shout(name string) {
    strings.ToUpper(name)
}
```

## divzero

Reports integer division of two integer literals whose quotient is zero. The spec's Arithmetic operators section defines integer division with x / y truncated towards zero.

Adapted from staticcheck's SA4025.

```
func f() int {
    return 1 / 2
}
```

## docstub

Reports a doc comment that only repeats the declaration's name. A doc comment should say what the declaration returns, does, or provides.

Adapted from go-critic's docStub.

```
// Foo does foo.
func Foo() {}
```

## docsyntax

Reports Markdown that Go doc comments do not implement: a code span, a fenced block, bold, or a link. A link is a bracketed name that does not resolve, followed by parentheses that hold nothing but a target, so a signature written after a name is not one. Go renders each of them as literal punctuation.

```
// F returns a `value`.
func F() {}
```

## dotimport

Reports a dot import in a file that is not a test. The dot form declares every exported identifier in the imported package's package block in the file block, so the file does not show where a name is defined. A dot import of internal/types/errors is not reported.

Inspired by revive's dot-imports.

```
package a

import . "strings"

func f(s string) bool {
    return Contains(s, "x")
}
```

## dubbitshift

Reports a shift of a fixed-size typed constant by a count at least as large as the type's width, or a shift of an unsigned typed constant whose result is zero below that width. A shift of a non-constant operand is left alone, and so is a shift with a platform-dependent type anywhere in its expression.

Adapted from staticcheck's SA9006.

```
const n = uint8(1) >> 8
```

## dupbranch

Reports an if whose two branches hold the same statements. The condition has no effect there, so either the branches were left half edited or one of them was meant to do something else.

Inspired by go-critic's dupBranchBody.

```
func f(ok bool) {
    if ok {
        done()
    } else {
        done()
    }
}
```

## dupbuildtag

Reports a // +build line whose condition an earlier line in the same file already gives. The repeated line adds nothing and is usually a copy-paste error.

Adapted from staticcheck's SA4019.

```
// +build linux
// +build linux

package a
```

## dupcase

Reports a case that names an expression an earlier case in the same switch named. The second case can never match, and the compiler folds only constants.

Inspired by go-critic's dupCase.

```
func f(xs []int) int {
    switch xs[0] {
    case xs[1]:
        return 1
    case xs[1]:
        return 2
    }
    return 0
}
```

## dupcutset

Reports a strings or bytes Trim, TrimLeft, or TrimRight call whose literal cutset repeats a character. A cutset is the set of code points to remove, so a repeat changes nothing; it is often a prefix or suffix the author meant to name.

Adapted from staticcheck's SA1024.

```
func trim(s string) string {
    return strings.Trim(s, "..")
}
```

## dupelseif

Reports an else-if whose condition repeats an earlier condition in the same chain, when each condition reads the same value both times and no init statement or condition that can read a different value lies between them. By the time the later one is reached, the earlier condition was false, so the repeated condition is false too and the branch can never run.

Inspired by staticcheck's SA4014.

```
func f(n int) int {
    if n > 0 {
        return 1
    } else if n > 0 {
        return 2
    }
    return 0
}
```

## dupimport

Reports one package imported twice under different names. Both names reach the same package, so a reader has to check which one a use site means.

Inspired by go-critic's dupImport.

```
import (
    "bytes"
    printing "bytes"
)
```

## duplarg

Reports a call that passes one identifier as both arguments of a function in a fixed set of library functions, where the answer cannot depend on two equal values. strings.Contains(s, s) is always true, math.Max(n, n) is n, and copy(xs, xs) copies a slice onto itself. A call on any other function is left alone.

Inspired by go-critic's dupArg.

```
func f(s string) bool {
    return strings.Contains(s, s)
}
```

## duplicatedcond

Reports an if whose condition repeats the condition of the if it is nested in. The rule reports when the inner if is the first statement of the outer body, has no init statement, and the condition reads the same value both times, so the inner test is true whenever the outer body runs.

```
func f(ready bool) {
    if ready {
        if ready {
            println("ready")
        }
    }
}
```

## dupoption

Reports an option that a later argument of the same call repeats. An option is an argument in a variadic tail whose element type is a function taking one parameter or more and returning nothing or one error. The rule assumes a function applies each option in order: a setting option's earlier assignment is overwritten, and one that appends runs twice.

Adapted from go-critic's dupOption.

```
func newAlpha() *server {
    return newServer("alpha",
        withName("alpha-one"),
        withTimeout(time.Second),
        withName("alpha-one"),
    )
}
```

## dupsubexpr

Reports a binary operator whose operands are the same value, as xs\[k] \< xs\[k]. One of them was meant to name something else. A NaN changes the answer of ==, !=, \<=, >=, - and /, so an operand that can hold one is left alone for those operators, and so is a call whose second read can differ.

Adapted from go-critic's dupSubExpr.

```
func f(xs []int, k int) bool {
    return xs[k] < xs[k]
}
```

## dynformat

Reports a printf-style call with a non-constant format string, no argument after it, and a plain counterpart. The value is used as a format string, so a percent sign in it begins a format operation and the value does not appear as written.

Adapted from staticcheck's SA1006.

```
func f(s string) {
    fmt.Printf(s)
}
```

## earlyreturn

Reports a return whose chain of conditions mixes && with || and does not fit one line. Early returns give each way out a guard and leave one answer at the end.

```
func f(s string) bool {
    return strings.Contains(s, "alpha") && strings.HasPrefix(s, "a") ||
        strings.Contains(s, "beta")
}
```

## elseif

Reports an if nested in an else block. Writing it as else if removes a pair of braces and one level of indent.

Inspired by go-critic's elseif.

```
func f(n int) int {
    if n == 0 {
        return 0
    } else {
        if n == 1 {
            return 1
        }
    }
    return -1
}
```

## embeddirective

Reports a //go:embed directive in a file that does not import embed, or one that does not precede a single variable of type string, \[]byte or embed.FS with no value.

Copied from gopls' embed, which is internal.

```
//go:embed static/index.html
var pages []string
```

## embedlit

Reports an embedded type named in a composite literal, or a field of it assigned just after, where Go 1.27 sets the promoted field in the outer literal instead.

Imported from golang.org/x/tools' modernize suite.

```
type item struct {
    base
}

type base struct {
    kind string
}

var first = item{base: base{kind: "alpha-one"}}
```

## emptybranch

Reports an if or else branch with no statements and no comment. An empty branch is often a placeholder or a leftover from an edit. A comment between the braces suppresses the report.

Adapted from staticcheck's SA9003.

```
func f(ready bool) {
    if ready {
    }
}
```

## emptycritical

Reports a lock taken and released with no statement between the two calls. The author often meant the Lock to cover the work below it, or wrote an Unlock without the defer they meant.

Adapted from staticcheck's SA2001.

```
func f(mu *sync.Mutex) {
    mu.Lock()
    mu.Unlock()
}
```

## emptydecl

Reports a var, const, type, or import block that declares nothing. The block was emptied and left behind.

Inspired by go-critic's emptyDecl.

```
var ()
```

## emptyfallthrough

Reports a case whose only statement is a fallthrough. Listing both values in one case does the same without the jump.

Inspired by go-critic's emptyFallthrough.

```
func f(n int) int {
    switch n {
    case 0:
        fallthrough
    case 1:
        return 1
    }
    return 0
}
```

## emptylines

Reports blank lines that separate nothing: one at the start or end of a block, and the second and later lines of a run inside one. gofmt keeps a blank line at an edge and prints at most one between two lines of content.

Adapted from revive's empty-lines.

```
func f(n int) {

    println(n)
}
```

## emptystring

Reports a test of whether a string is empty, written as a comparison of its length with zero. A comparison with the empty string is the same test, and the form a reader expects.

Inspired by go-critic's emptyStringTest.

```
func f(s string) bool {
    return len(s) == 0
}
```

## encodeoverlap

Reports a call to an encoder in encoding/base64, encoding/base32, encoding/hex, or encoding/ascii85 whose destination and source are slices of one backing array that start at the same index. The encoders write to the destination before reading the next source bytes, so the source can be overwritten before it is read.

Inspired by staticcheck's SA1031.

```
func encode(dst []byte) {
    base64.StdEncoding.Encode(dst, dst)
}
```

## equalfold

Reports a comparison of two values converted to the same case. Each side builds a copy, where strings.EqualFold or bytes.EqualFold compares the originals without one.

Adapted from go-critic's equalFold.

```
func f(a, b string) bool {
    return strings.ToLower(a) == strings.ToLower(b)
}
```

## errcheck

Reports a call whose error is discarded, so the failure goes unseen. Writes to bytes.Buffer, strings.Builder, or hash.Hash are left alone, as are fmt's Print family, its Fprint family to bytes.Buffer, strings.Builder, or standard error, a Read from crypto/rand or math/rand, io.Pipe's CloseWithError, an error assigned to the blank identifier, and a deferred call that is not a function literal. An error whose type is a type parameter, an alias of error, or an unnamed interface is not reported.

Imported from lesiw.io/errcheck, a fork of kisielk's errcheck.

```
func save(path string) {
    os.WriteFile(path, nil, 0o644)
}
```

## errflow

Reports a function that writes the same error path two ways: an else block after a branch that exits, a nil check that only returns the value it tested, or a last call whose error no guard checks. A function with a single check returns the call directly; a function with more than one check guards every step.

Inspired by revive's indent-error-flow and if-return.

```
func f(err error) int {
    if err != nil {
        return 0
    } else {
        return 1
    }
}
```

## errfmt

Reports an error built with errors.New whose message is a literal, where rewriting it to fmt.Errorf needs no import and leaves errors unused. Reports an error formatted with %v or %s where %w would wrap it. Only a literal format string and an argument whose static type is error are considered.

```
func f(err error) error {
    return fmt.Errorf("open failed: %v", err)
}
```

## errisnil

Reports an error compared with nil twice in one boolean expression, as err != nil && err != nil. Both comparisons read the same value, so the second changes nothing.

Adapted from staticcheck's SA4000 family.

```
func f(err error) bool {
    return err != nil && err != nil
}
```

## errname

Reports a sentinel error whose name does not begin with Err, and an error type whose name does not end in Error. The names are how a reader tells the two apart.

Imported from Antonboom's errname.

```
var NotFound = errors.New("not found")
```

## errnotlast

Reports a function whose last error result is followed by another result; the error goes last, so a caller writes value, err := fetch(). An error result is the predeclared error, or a type that implements error and whose name, or whose pointee's name, ends in Error after aliases are resolved; a type parameter is left alone, and a final result that implements error is accepted. The signature of a function declaration, a method declaration or a function literal is checked; every other function type is left to errorlast.

Adapted from revive's error-return.

```
func lookup() (error, string) {
    return nil, ""
}
```

## errorlast

Reports a signature whose error result is not the last one. A caller writes the error last, as in n, err := lookup(). A result is an error when its type is the predeclared error or implements error under a name ending in Error.

Adapted from revive's error-return.

```
func lookup() (error, int) {
    return nil, 0
}
```

## errorsas

Reports a call to errors.As whose target is not a non-nil pointer to a type that implements error, and a target of type \*error.

Imported from the x/tools vet suite.

```
func f(err error) {
    var target os.PathError
    errors.As(err, target)
}
```

## errorsastype

Reports an errors.As call on a pointer target declared just above it, where errors.AsType gives the same test as a typed value and a bool.

Imported from golang.org/x/tools' modernize suite.

```
func f(err error) {
    var target *os.PathError
    if errors.As(err, &target) {
        use(target)
    }
}
```

## errorsisorder

Reports a call to errors.Is whose first argument is a package-level error variable from another package. A leaf sentinel with no Is method in the first position makes the call match only the sentinel itself. The suggested fix swaps the arguments, which changes behavior.

Adapted from staticcheck's SA1032.

```
func f(err error) bool {
    return errors.Is(errors.ErrUnsupported, err)
}
```

## errshadow

Reports a declaration directly in an if body that shadows an error variable the condition tested. A later use of err denotes the inner variable. The rule leaves alone a declaration in a nested bare block, a plain else block, or a loop header; a declaration that introduces another variable; and a declaration in a body whose condition requires the error to be nil through direct nil comparisons and the boolean operators.

Inspired by staticcheck's SA4006 family of forgotten-error checks; the shadow lookup follows the shadow analyzer in golang.org/x/tools.

```
func f() error {
    if err := g(); err != nil {
        err := h()
        return err
    }
    return nil
}
```

## errstrings

Reports an error string that starts with a capital letter or ends with punctuation. The message usually follows other text, as in reading config: , so a capital inside it reads as a sentence break the caller did not write.

Inspired by revive's error-strings.

```
var errMissing = errors.New("Not found")
```

## errwrapdirect

Reports a fmt.Errorf call whose format is exactly %w. The call adds no text, so an ordinary error can stand in its place. An argument whose static type implements fmt.Formatter is left alone, and a wrapper around a sentinel is reported even though only the replacement equals the sentinel.

Adapted from staticcheck's S1028.

```
func wrap(err error) error {
    return fmt.Errorf("%w", err)
}
```

## errwrapnil

Reports a fmt.Errorf call whose %w verb has a nil error operand. The nil operand does not contribute a cause, and the message contains %!w(\<nil>) where the cause would be.

Adapted from this suite's nilerrwrap, itself inspired by the x/tools printf analyzer.

```
func load() error {
    return fmt.Errorf("fetch: %w", nil)
}
```

## evalorder

Reports a return that reads a value and passes its address to a call in the same statement. The spec fixes the order of neither, so the read may see the value before or after the call writes it.

Adapted from go-critic's evalOrder.

```
func f(n int) (int, int) {
    return n, bump(&n)
}
```

## exposedsyncmutex

Reports a sync.Mutex or sync.RWMutex embedded in a type an importer can reach. The embed puts Lock and Unlock in the method set of the type or its pointer, so a caller can unlock the mutex a method holds, and a caller that holds the lock deadlocks the next method that takes it.

Adapted from go-critic's exposedSyncMutex.

```
type Cache struct {
    sync.Mutex
}
```

## fieldcopy

Reports a struct literal that copies every field of a same-shaped struct whose tags differ. The spec ignores tags when it allows the conversion.

Adapted from staticcheck's S1016, which ignores tags.

```
type first struct {
    Kind string `json:"kind"`
}

type second struct {
    Kind string `json:"type"`
}

func f(v first) second {
    return second{Kind: v.Kind}
}
```

## finalizercapture

Reports a runtime.SetFinalizer call whose finalizer captures the object through its own variable or bound receiver. The object is reachable from the finalizer, so it may not be collected and the finalizer may not run.

Adapted from staticcheck's SA5005.

```
func f() {
    v := new(int)
    runtime.SetFinalizer(v, func(*int) { println(v) })
}
```

## findallzero

Reports a FindAll call on a regexp.Regexp whose count is zero. A count of n returns at most n matches and a negative count returns every match, so zero returns none.

Inspired by staticcheck's SA1010.

```
func f(s string) []string {
    return regexp.MustCompile("a+").FindAllString(s, 0)
}
```

## flagderef

Reports a flag value read from the pointer that flag.Bool and its siblings return. Before Parse the pointer holds the default, so a read gets the default rather than the flag the user set.

Inspired by go-critic's flagDeref.

```
func f() {
    _ = *flag.String("name", "", "a name")
}
```

## flagname

Reports a flag name that begins with a hyphen or contains an equal sign. FlagSet.Var has panicked on such a name since Go 1.17, and a declaration that never runs registers a flag no command line can set.

Adapted from go-critic's flagName.

```
var level = flag.Int("-level", 0, "")
```

## fortrue

Reports a for statement whose condition is the literal true and whose for clause has no init or post statement. An omitted condition means true, so the loop runs the same without the condition.

Adapted from staticcheck's S1006.

```
func f() {
    for true {
    }
}
```

## forvar

Reports a range loop that copies its loop variable into a fresh one with x := x. Since Go 1.22 each iteration has its own variable, so the copy does nothing.

Imported from golang.org/x/tools' modernize suite.

```
func f(xs []int) {
    for _, x := range xs {
        x := x
        use(x)
    }
}
```

## framepointer

Reports assembly that writes the frame pointer in a function that has not saved it. The runtime walks the stack through that register.

Imported from the x/tools vet suite.

```
TEXT ·compare(SB), NOSPLIT, $0
    MOVQ 8(SP), BP
    RET
```

## gofixdirective

Reports a //go:fix inline directive on a declaration that gives the toolchain nothing to substitute, as a constant whose value is a literal rather than another named constant.

Imported from the x/tools vet suite.

```
//go:fix inline
const answer = 42
```

## gospell

Reports an English word spelled as Go does not spell it. Go writes American English, and the standard library shows it: context.Canceled has one l. The rule reads comments and string literals; an identifier may be a name from another package.

```
// The behaviour is deliberate.
func f() {}
```

## headercanon

Reports a non-canonical constant key used to index or delete from an http.Header map. Set, Add, Get, Values, and Del canonicalize their key; a direct key is used as written, so a later Get canonicalizes the key and returns the zero value. A key in a variable is left alone, so a non-canonical key can still be used through one.

Adapted from staticcheck's SA1008.

```
func f(h http.Header) {
    h["content-type"] = nil
}
```

## hostport

Reports an address built with fmt.Sprintf from a host and a port and passed to net.Dial. A bare IPv6 host needs brackets in that string, and net.JoinHostPort writes them.

Imported from the x/tools vet suite.

```
func dial(host string) (net.Conn, error) {
    return net.Dial("tcp", fmt.Sprintf("%s:%d", host, 80))
}
```

## hostportconst

Reports a constant address that a listen function cannot parse as host:port.

Adapted from staticcheck's SA1020, which checks only the net/http functions.

```
func listen() (net.Listener, error) {
    return net.Listen("tcp", "alpha-one")
}
```

## httpmux

Reports a ServeMux pattern that names a method or a wildcard in a module that declares Go 1.21 or earlier. The pattern registers in that release and does not match what it was written for.

Imported from the x/tools vet suite.

```
func routes(mux *http.ServeMux) {
    mux.HandleFunc("GET /items", list)
}
```

## httpresponse

Reports a read of an http.Response whose error has not been tested, as a deferred Close on a response that may be nil. The error says whether the response exists.

Imported from the x/tools vet suite.

```
func head(url string) error {
    resp, err := http.Head(url)
    defer resp.Body.Close()
    return err
}
```

## identicalconds

Reports an else-if that repeats an earlier condition in the same chain, when no init statement or value-changing condition lies between them. The earlier condition is false at the later test, so the branch can never run.

Adapted from revive's identical-ifelseif-conditions.

```
func f(n int) int {
    if n > 0 {
        return 1
    } else if n > 0 {
        return 2
    }
    return 0
}
```

## ifaceassert

Reports a type assertion to an interface that the source interface cannot implement, because a method of the same name has a different signature. The assertion can never hold.

Imported from the x/tools vet suite.

```
func f(v interface{ Read() }) {
    _ = v.(io.Reader)
}
```

## ifelsechain

Reports an if-else chain of four or more branches. A switch puts each condition at one indent, where a reader looks for it.

Inspired by go-critic's ifElseChain.

```
func f(n int) int {
    if n == 0 {
        return 0
    } else if n == 1 {
        return 1
    } else if n == 2 {
        return 2
    } else if n == 3 {
        return 3
    }
    return -1
}
```

## ifreturn

Reports an else block after an if whose body only returns. The else runs only when the condition is false, so its statements can follow the if at the same indent.

This is the return-only case of errflow's report, adapted from revive's indent-error-flow.

```
func f(n int) int {
    if n == 0 {
        return 0
    } else {
        return 1
    }
}
```

## importcomment

Reports a package clause with an import comment naming the canonical path. The go command reads the comment in GOPATH mode only, so a module ignores it.

Imported from golang.org/x/tools' modernize suite.

```
package p // import "example.com/p"
```

## importgroup

Reports an import block that is not grouped standard library, other modules, then the packages the -importgroup.local flag names, one blank line between the groups and each group sorted by path. Without a prefix there is no third group, so the rule reports nothing.

```
import (
    "os"

    "fmt"
)
```

## importshadow

Reports a declaration that shadows an imported package name. A reader who misses the declaration reads every later reference as the package.

Inspired by go-critic's importShadow.

```
import "path"

func join(dir, name string) string {
    return path.Join(dir, name)
}

func clean(path string) string {
    return path
}
```

## increment

Reports an increment or decrement written as a compound assignment. The spec gives i++ and i += 1 the same meaning, and Go code writes i++.

Adapted from revive's increment-decrement.

```
func f(i int) int {
    i += 1
    return i
}
```

## indexalloc

Reports a byte slice converted to a string only to be searched. The conversion copies the bytes, and the bytes package searches the slice in place.

Inspired by go-critic's indexAlloc.

```
func f(b []byte) int {
    return strings.Index(string(b), "alpha")
}
```

## indexcontains

Reports a test of a strings.Index or bytes.Index result against -1. A Contains call returns the same boolean, negated when the test is for an absent substring.

Adapted from staticcheck's S1003.

```
func has(s, sub string) bool {
    return strings.Index(s, sub) != -1
}
```

## ineffectualcall

Reports a call written as a statement to a method that has a value receiver and whose body is one return of an effect-free value. The receiver is a copy, and the result is not used, so the call has no effect.

Adapted from staticcheck's SA4017.

```
type point struct{ x int }

func (p point) X() int { return p.x }

func f(p point) {
    p.X()
}
```

## initclause

Reports an if or switch init clause whose statement declares nothing. Effective Go describes the clause as the place to set up a local variable, so a call there reads as something else.

Inspired by go-critic's initClause.

```
func f(m map[string]int, k string) bool {
    if delete(m, k); len(m) == 0 {
        return true
    }
    return false
}
```

## initialisms

Reports a name whose initialism is written in mixed case. Go writes an initialism in one case throughout a name: URLPath and urlPath, never UrlPath.

Inspired by golint's commonInitialisms.

```
type UrlPath struct{}
```

## ipliteral

Reports an IP address written into source as part of a URL, a host:port, or a user@host. An address in a string cannot move to DNS or fail over, and loopback and the unspecified address are left alone. A test file is left alone too, since a fixture names the place it talks to on purpose.

```
func dial() string {
    return "203.0.113.9:22"
}
```

## isnilbool

Reports a comparison of a comparison with true using == or with false using !=, as in (a == b) == true. The inner comparison already has the value, so the outer comparison adds nothing.

Adapted from staticcheck's S1002.

```
func f(a, b int) bool {
    return (a == b) == true
}
```

## joinseparator

Reports a literal element ending with a redundant separator in a path.Join or filepath.Join call. Join cleans the result, so an element's trailing separator changes nothing unless it completes a root. On Windows a filepath.Join element is reported only when every argument is constant, because a trailing separator there can complete a share root.

Adapted from go-critic's filepathJoin, which reports a literal argument that holds a separator.

```
func config() string {
    return filepath.Join("dir", "alpha/")
}
```

## jsonrecv

Reports MarshalJSON on a pointer receiver that does not need the pointer, and UnmarshalJSON on a value receiver that writes directly to its copy. The first leaves MarshalJSON out of the value's method set, and the second's write never reaches the decoded value.

Adapted from revive's marshal-receiver.

```
func (i *item) MarshalJSON() ([]byte, error) {
    return nil, nil
}
```

## lenzero

Reports a comparison of len with zero that is always true or always false. len returns a count, and a count is never negative.

Inspired by go-critic's sloppyLen.

```
func f(xs []int) bool {
    return len(xs) < 0
}
```

## linelen

Reports a line wider than the limit, counted in columns with a tab as four. An import, a generated file, and a line whose width is mostly one string literal or URL are exempt.

A function signature is exempt too, because a function type is never broken across lines. Only the signature's own text is excused: a line that a call around it makes too long is still reported.

```
func f() {
    fmt.Println("alpha", "bravo", "charlie", "delta")
}
```

## linewrap

Reports an if, for, or switch header that spans more than one line. The opening brace of the body sits on the line the statement begins, and a header spread over several lines puts the branch point under a block of arguments.

```
func f(ctx context.Context, arg int) {
    if err := call(
        ctx, arg); err != nil {
        return
    }
}
```

## loopclosure

Reports a function literal that reads a loop variable of an enclosing loop and is called in a way that outlives the iteration, as a go or defer statement does. In a file that declares a Go version before 1.22 every iteration shares the variable, so the literal can read a value its own iteration never had.

Imported from the x/tools vet suite.

```
//go:build go1.21

package p

func f(xs []int) {
    for _, x := range xs {
        go func() {
            println(x)
        }()
    }
}
```

## looponce

Reports a loop that runs at most once: its body has a top-level control statement and ends in a return or a break.

Adapted from staticcheck's SA4004.

```
func f(x int) int {
    for {
        if x > 0 {
            return x
        }
        return 0
    }
}
```

## loopvarcond

Reports a for loop whose condition tests a variable the loop never changes. The condition would evaluate to the same value at every test, so a loop cannot end through a change in its condition.

Adapted from staticcheck's SA4008.

```
func f(i, n int) {
    for i < n {
        use(i)
    }
}
```

## lostcancel

Reports a context cancel function that is never called. The context and everything derived from it stay live until its parent is canceled.

Imported from the x/tools vet suite.

```
func f(ctx context.Context) context.Context {
    next, _ := context.WithCancel(ctx)
    return next
}
```

## lowerfold

Reports a comparison of two strings converted to lower or upper case, as strings.ToLower(a) == strings.ToLower(b). strings.EqualFold compares without building either converted string, and the fix writes it.

Adapted from staticcheck's SA6005.

```
func f(a, b string) bool {
    return strings.ToLower(a) == strings.ToLower(b)
}
```

## makecap

Reports a make call whose capacity argument is redundant: a slice whose capacity has the same value as its length, or a channel whose size is the constant zero.

Adapted from staticcheck's S1019.

```
func sized(n int) []int {
    return make([]int, n, n)
}
```

## makezero

Reports a slice made with a non-zero constant length by := and then appended to. The append writes past the elements the make left there, and they stay in the result.

Inspired by golangci-lint's makezero.

```
func f() []int {
    xs := make([]int, 4)
    return append(xs, 1)
}
```

## manualtrim

Reports an if statement that trims a string prefix by reslicing it. strings.TrimPrefix returns the string unchanged when the prefix is absent, so the if statement becomes a single assignment.

Inspired by staticcheck's S1017.

```
func f(s, prefix string) string {
    if strings.HasPrefix(s, prefix) {
        s = s[len(prefix):]
    }
    return s
}
```

## mapfind

Reports a range over a map whose body searches for one key. An index expression reaches that entry directly, where the range visits every key.

Adapted from revive's inefficient-map-lookup.

```
func f(m map[string]int, want string) (int, bool) {
    for k, v := range m {
        if k == want {
            return v, true
        }
    }
    return 0, false
}
```

## mapkey

Reports a map key that is unhashable. A map keyed by an interface accepts a slice, a map, or a function, and the map panics when it hashes one. For a conversion to an interface the report names the operand's type, so any(xs) names \[]int.

The name comes from go-critic's mapKey.

```
func f(m map[any]int, xs []int) {
    m[xs] = 1
}
```

## maplenloop

Reports a call to len on the map a range statement iterates, when nothing in the loop changes the map. The length is the same at every pass.

```
func f(m map[string]int) {
    for range m {
        use(len(m))
    }
}
```

## maplookupinloop

Reports a map lookup in a loop that reads the same map with a key that does not change. The lookup returns the same value on every pass.

Adapted from staticcheck's SA6001.

```
func f(m map[string]int, keys []string) {
    for range keys {
        _ = m["alpha"]
    }
}
```

## mapsloop

Reports a range over a map or an iter.Seq2 whose body copies one entry into another map. maps.Copy writes the map loop as one call, maps.Insert the iterator loop, in a package that declares Go 1.23 or later.

Imported from golang.org/x/tools' modernize suite.

```
func f(dst, src map[string]int) {
    for k, v := range src {
        dst[k] = v
    }
}
```

## marshalfunc

Reports a call to encoding/json that marshals a value whose type contains a channel or a func. The call compiles; Marshal returns an UnsupportedTypeError at run time when it encodes a channel or func.

Inspired by staticcheck's SA1026.

```
func f(c chan int) {
    json.Marshal(c)
}
```

## marshalrecv

Reports a write to the receiver of MarshalJSON, MarshalText, MarshalBinary, or GobEncode declared with a value receiver. The method works on a copy, so the write never reaches the caller.

Inspired by revive's modifies-value-receiver.

```
func (i item) MarshalText() ([]byte, error) {
    i.text = nil
    return nil, nil
}
```

## mathintconv

Reports a math.Ceil, math.Floor, math.Round, math.RoundToEven, or math.Trunc call on an integer converted to a float. Each rounding function keeps an integer value unchanged, so the call returns its argument.

Adapted from staticcheck's SA4015.

```
func f(n int) float64 {
    return math.Ceil(float64(n))
}
```

## mathrandseed

Reports a call to the package-level math/rand.Seed, deprecated as of Go 1.20 because the global generator is seeded automatically.

Adapted from staticcheck's SA1019.

```
func seeded() {
    rand.Seed(1)
}
```

## methodexprcall

Reports a method expression called with its receiver. The method call names the receiver once, where the expression names it again as the argument.

Adapted from go-critic's methodExprCall.

```
func f(b *strings.Builder) string {
    return (*strings.Builder).String(b)
}
```

## minmax

Reports a test that picks one of two values for a variable, which min or max writes on one line. A floating-point operand is left alone, where the two forms differ at a NaN.

Imported from golang.org/x/tools' modernize suite.

```
func f(a, b int) int {
    x := a
    if a < b {
        x = b
    }
    return x
}
```

## modifiesparam

Reports a write to a parameter or a receiver that no read follows. Both are a copy the caller does not see.

Adapted from revive's modifies-parameter.

```
func normalize(name string) {
    name = strings.TrimSpace(name)
}
```

## modone

Reports a binary remainder operation whose divisor is a constant expression with value one and no identifier. The result is zero for every integer left operand.

Adapted from staticcheck's SA4028.

```
func f(n int) int {
    return n % 1
}
```

## nancompare

Reports a comparison with math.NaN(). Under IEEE 754 a NaN is unequal to every value, including itself, and every ordering comparison with one is false, so the comparison has a constant answer. A case in an expression switch that compares the tag with NaN is reported, and so is a switch whose tag is NaN; no case can match.

Adapted from staticcheck's SA4012.

```
func stopped(v float64) bool {
    return v == math.NaN()
}
```

## nanosleep

Reports a bare small integer passed to time.Sleep as a number of nanoseconds.

Inspired by staticcheck's SA1004.

```
func pause() {
    time.Sleep(1)
}
```

## negatedbool

Reports a boolean expression that a negation makes constant or redundant: a conjunction of an expression with its negation, the matching disjunction, or a double negation that cancels. The conjunction is always false and the disjunction is always true.

Adapted from staticcheck's SA4013, which reports a double negation.

```
func f(ok bool) bool {
    return !ok && ok
}
```

## negzero

Reports a unary minus on a constant zero of a floating-point or complex type, including a type parameter restricted to those types. An explicit conversion of a typed integer constant zero to one of those types is reported too. The spec says there are no constants denoting the IEEE 754 negative zero, so the minus is not part of the value.

Adapted from staticcheck's SA4026.

```
const x = -0.0
```

## newderef

Reports a dereference of a fresh allocation. new(T) zeroes a T and the dereference reads the zero back, which T{} writes directly.

Inspired by go-critic's newDeref.

```
func f() int {
    return *new(int)
}
```

## newexpr

Reports a function whose body only returns the address of its parameter. Go 1.26 gives new the same call, and the fix marks the function for inlining.

Imported from golang.org/x/tools' modernize suite.

```
func ptr(v int) *int {
    return &v
}
```

## nilchansend

Reports a send or receive on a channel that is nil at that point. The operation blocks forever.

Adapted from staticcheck's SA5011 family.

```
func f() {
    var ch chan int
    ch <- 1
}
```

## nilchecklen

Reports a nil test joined to a comparison of len that already answers it. The length of a nil slice, map, or channel is zero.

Inspired by staticcheck's S1009.

```
func f(xs []int) bool {
    return xs != nil && len(xs) > 0
}
```

## nilerrwrap

Reports a fmt.Errorf call whose %w verb has no error to wrap: the argument is missing, is not an error, or is nil. The errors package documentation states that a nil error returned from Unwrap means the error wraps nothing.

Inspired by the x/tools printf analyzer, which reports the missing and non-error operands; this rule adds the nil error.

```
func load() error {
    return fmt.Errorf("read: %w", nil)
}
```

## nilfunc

Reports a comparison of a function with nil. A function declared in the source is never nil, so the comparison is always false.

Imported from the x/tools vet suite.

```
func f() {}

func g() bool {
    return f == nil
}
```

## nilfuncall

Reports a call of a function value that the same block set to nil, which panics. The suite's nilness pass reports most of the same calls from SSA; it misses a package-level literal and a literal that captures the variable.

```
func f() {
    h := func() {}
    h = nil
    h()
}
```

## nilinterfacecmp

Reports a comparison of an interface value that holds a typed nil with nil. Assigning a nil value of a concrete type to an interface stores that type, so the interface is not nil.

Adapted from staticcheck's SA4023.

```
func f() bool {
    var p *int
    var v any = p
    return v != nil
}
```

## nilmapassign

Reports an assignment to an element of a nil map, which panics at run time. The map has to be a nil literal or a local variable whose value is nil on every path to the write.

Adapted from staticcheck's SA5000.

```
func f() {
    var m map[string]int
    m["x"] = 1
}
```

## nilness

Reports a nil test whose answer the assignments already fix, as a pointer known to be nil or known to be set, and a dereference of a nil pointer.

Imported from the x/tools vet suite.

```
func f() {
    var p *int
    if p != nil {
        use(p)
    }
}
```

## nilrange

Reports a nil check that wraps a range loop over a slice or a map. Ranging over a nil slice or map runs the body zero times, so the check is redundant.

Adapted from staticcheck's S1031.

```
func walk(xs []int) {
    if xs != nil {
        for range xs {
        }
    }
}
```

## nilreturn

Reports a return of an error the if just found to be nil. The return always yields nil, so either the test is inverted or the return should be nil. A nil guard on another type is left alone.

Inspired by go-critic's nilValReturn.

```
func f(err error) error {
    if err == nil {
        return err
    }
    return nil
}
```

## nilsliceappend

Reports an append to a nil slice where a slice literal gives the result. The appended values become the elements, and the result is a fresh slice either way.

Adapted from staticcheck's S1019.

```
func f() []int {
    return append([]int(nil), 1, 2)
}
```

## niltimer

Reports a call to a method of time.Timer or time.Ticker on a nil pointer. A nil receiver panics.

Adapted from staticcheck's SA1019 family.

```
func f() {
    var t *time.Timer
    t.Stop()
}
```

## nilwrite

Reports a Write method with io.Writer's signature that may modify the byte slice it was given, as p\[0] = 0 or p = append(p, 0). io.Writer's doc says the method must not modify the slice data, even temporarily.

Adapted from staticcheck's SA1023.

```
func (sink) Write(p []byte) (int, error) {
    p[0] = 0
    return len(p), nil
}
```

## noopbits

Reports a bitwise AND, OR, or XOR whose right operand is a zero integer literal or an iota zero constant. The zero operand leaves the bits of the other operand unchanged under OR and XOR; an AND with zero clears every bit.

Adapted from staticcheck's SA4016.

```
func f(n int) int {
    return n | 0
}
```

## nowsub

Reports time.Now().Sub(t) or t.Sub(time.Now()). time.Since and time.Until are shorthand for these two calls. In the Since direction a call is reported only when the argument does no work, because the rewrite moves the clock read past the argument.

Adapted from staticcheck's S1012 and S1024.

```
func f(start time.Time) time.Duration {
    return time.Now().Sub(start)
}
```

## octal

Reports an octal literal written with a leading zero. The 0o prefix marks the base where a leading zero reads as padding. An argument of time.Date keeps its zero, and so does a literal of zero, or one written 0\_755.

Inspired by go-critic's octalLiteral.

```
const mode = 0755
```

## offbyone

Reports an index at the length of the value it indexes, and a slice whose only bound is a strings.Index or bytes.Index result. The last element sits at one less, and Index returns -1 when the substring is absent, so the index or slice can be out of range.

Inspired by go-critic's offBy1.

```
func f(xs []int) int {
    return xs[len(xs)]
}
```

## omitzero

Reports an omitempty tag on a struct-typed field. A struct value never encodes as empty, so the tag does nothing; omitzero, added in Go 1.24, omits the field when it is zero.

Imported from golang.org/x/tools' modernize suite.

```
type label struct {
    kind string
}

type item struct {
    label label `json:"label,omitempty"`
}
```

## osremoveall

Reports os.Remove or os.RemoveAll on a path that a variable leaves at the filesystem root, as dir + "/" does when dir is empty or the root. For that path, a guard that proves dir is not empty and filepath.Clean(dir) is not the root exempts the call.

Adapted from staticcheck's SA9007.

```
func removeDir(dir string) {
    os.RemoveAll(dir + "/")
}
```

## packagename

Reports a package name that gives no sense of the contents, a package called pkg in a directory with another name, a name with an underscore, or a name with a capital letter. The package name prefixes every exported identifier a client uses, so a name that gives no sense of the contents is read at every use.

Adapted from revive's package-naming.

```
package util
```

## panicnil

Reports a call to panic with the predeclared nil. The call passes nil, so the crash report can say only that the argument was nil.

```
func at(xs []int, i int) int {
    if i < 0 || i >= len(xs) {
        panic(nil)
    }
    return xs[i]
}
```

## plusbuild

Reports a // +build line in a file that also holds a //go:build line. The older line is obsolete since Go 1.18, and gofmt removes it.

Imported from golang.org/x/tools' modernize suite.

```
//go:build linux
// +build linux

package p
```

## poolarg

Reports a value stored in a sync.Pool that is not one pointer word. The compiler stores a value in the data word only when the value is one pointer word, and points the data word at memory holding any other value.

Adapted from staticcheck's SA6002.

```
func keep(p *sync.Pool, b []byte) {
    p.Put(b)
}
```

## predictivetemp

Reports a file created at a fixed path in a shared directory. Every user can write there, so another process may already have created the file this call opens.

Adapted from gosec's G303.

```
func f() (*os.File, error) {
    return os.Create("/tmp/report")
}
```

## preferdecoderune

Reports a string converted to a rune slice to read its first rune. The conversion decodes every rune, where utf8.DecodeRuneInString decodes one.

Adapted from go-critic's preferDecodeRune.

```
func f(s string) rune {
    return []rune(s)[0]
}
```

## preferfprint

Reports a formatted string built with fmt and then written to a writer. fmt.Fprintf formats into the writer, so the string in between is never built.

Inspired by go-critic's preferFprint.

```
func f(b *bytes.Buffer, n int) {
    b.WriteString(fmt.Sprintf("%d", n))
}
```

## preferstringwriter

Reports a string converted to a byte slice and then written to a writer whose type declares WriteString. The conversion copies the string, and WriteString takes it as it is.

Adapted from go-critic's preferStringWriter.

```
func f(b *bytes.Buffer, s string) {
    b.Write([]byte(s))
}
```

## preferwritebyte

Reports a one-element byte slice literal written to a writer, and that literal converted to a string. WriteByte takes the byte, so the literal is never built.

Inspired by go-critic's preferWriteByte.

```
func f(b *bytes.Buffer) {
    b.Write([]byte{'\n'})
    b.WriteString(string([]byte{'0'}))
}
```

## printf

Reports a format string whose verbs and arguments disagree, and a formatting call with the wrong number of arguments. The check covers fmt, log, and a wrapper of either.

Imported from the x/tools vet suite.

```
func f(n int) string {
    return fmt.Sprintf("%d", "one")
}
```

## queryadd

Reports an Add call on the values returned by URL.Query. Query builds a separate map from RawQuery, so the added value is discarded and RawQuery is unchanged.

Inspired by staticcheck's SA4027.

```
func add(u *url.URL) {
    u.Query().Add("some-key", "some-value")
}
```

## randzero

Reports a call to math/rand or math/rand/v2 whose upper bound is one. A bound of 1 leaves 0 as the only value, so the call always returns 0.

Adapted from staticcheck's SA4030, extended to constant expressions.

```
func f() int {
    return rand.Intn(1)
}
```

## rangeappendall

Reports a range loop that appends each element of a slice one at a time. Each append may grow the slice and copy it; one append with the whole slice grows it once.

Inspired by go-critic's rangeAppendAll, which reports the opposite loop.

```
func merge(dst, src []string) []string {
    for _, s := range src {
        dst = append(dst, s)
    }
    return dst
}
```

## rangeint

Reports a three-clause for loop that counts to an integer and leaves its counter alone in the body. Go 1.22 writes the same loop with a range.

Imported from golang.org/x/tools' modernize suite.

```
func f(n int) {
    for i := 0; i < n; i++ {
        use(i)
    }
}
```

## rangelast

Reports a range loop that keeps only the last element of a slice or array. The loop assigns every element in turn, and the last assignment sets the value.

The rule is original to this suite.

```
func f(xs [3]int) (last int) {
    for _, v := range xs {
        last = v
    }
    return last
}
```

## rangestring

Reports a range over a string that only counts runes. utf8.RuneCountInString returns that count.

```
func count(s string) int {
    n := 0
    for range s {
        n++
    }
    return n
}
```

## rangevaladdress

Reports an address taken from a range loop's variable and kept past the iteration. In a file that declares a Go version before 1.22 every iteration shares the variable, so a pointer kept past the iteration reads the value the loop wrote last, and a range clause that assigns its variables with = shares them at every version.

Adapted from revive's range-val-address.

```
//go:build go1.21

package p

func f(ns []int) (out []*int) {
    for _, n := range ns {
        out = append(out, &n)
    }
    return out
}
```

## recursiveiter

Reports an iterator that ranges over a call to itself. Each nested range-over-func typically heap-allocates a variable, and each element passes through a dynamic yield call per enclosing range.

Vendored from gopls' recursiveiter, which is internal.

```
func (l *list) All() iter.Seq[int] {
    return func(yield func(int) bool) {
        for range l.next.All() {
        }
    }
}
```

## recvname

Reports a method receiver whose name differs from the name another method of the type uses, and one named this, self, or me. A reader who sees two names has to check whether the second is a different value.

Inspired by revive's receiver-naming.

```
func (c *client) get() {}

func (cl *client) post() {}
```

## recvtype

Reports a type whose methods mix pointer and value receivers. A value receiver works on a copy, and one kind for the whole type settles which methods a value has.

Inspired by golangci-lint's recvcheck.

```
func (p *pair) swap() {}

func (p pair) at() {}
```

## redundantalias

Reports an import alias that repeats the name the import already gets. The spec makes the name in the imported package's clause the default.

Inspired by revive's redundant-import-alias.

```
import (
    fmt "fmt"
)
```

## redundantbuildtag

Reports a //go:build line the file name already implies. The go command includes a file only where the tags in its name hold.

Adapted from revive's redundant-build-tag.

```
// a_linux.go
//go:build linux

package a
```

## redundantheaderkey

Reports a call to http.CanonicalHeaderKey on the key argument of an http.Header method. Add, Set, Get, Del, and Values canonicalize the key themselves, so the call is redundant.

Adapted from staticcheck's S1035.

```
func f(h http.Header) {
    h.Get(http.CanonicalHeaderKey("some-key"))
}
```

## redundantreturn

Reports a return that ends the body of a function declaring no results. The function ends either way, so the return has no effect.

Adapted from the return check in staticcheck's S1023.

```
func f() {
    return
}
```

## redundantsprint

Reports a fmt.Sprint whose operand has static type string, and a string conversion around one. Both return the operand unchanged. A named string type is left alone, since Sprint may call its String method.

Adapted from go-critic's redundantSprint.

```
func f(s string) string {
    return fmt.Sprint(s)
}
```

## reflecttypeassert

Reports a two-value type assertion on the result of reflect.Value.Interface. reflect.TypeAssert asserts on the value itself, without the interface in between.

Imported from golang.org/x/tools' modernize suite.

```
func f(v reflect.Value) (string, bool) {
    return v.Interface().(string)
}
```

## reflecttypefor

Reports a reflect.TypeOf call that names a type. reflect.TypeFor takes that type as a type argument and reads the same, from Go 1.22.

Imported from golang.org/x/tools' modernize suite.

```
func f() reflect.Type {
    return reflect.TypeOf(uint32(0))
}
```

## reflectvaluecompare

Reports a comparison of two reflect.Value values, with == or reflect.DeepEqual. Both compare the reflect package's representation in place of the values inside.

Imported from the x/tools vet suite.

```
func f(a, b reflect.Value) bool {
    return a == b
}
```

## regexpconst

Reports a regexp pattern that does not compile, where the pattern is a constant. MustCompile panics and the other functions return the error on every run.

Adapted from staticcheck's SA1000.

```
var re = regexp.MustCompile("a(")
```

## regexpmatchloop

Reports a call to regexp.Match, regexp.MatchString or regexp.MatchReader in a loop body, where the pattern is a constant. Each call compiles the pattern, and the pattern does not change between calls, so one compile before the loop suffices.

Adapted from staticcheck's SA6000.

```
func f(ss []string) {
    for _, s := range ss {
        regexp.MatchString("a+", s)
    }
}
```

## regexpmust

Reports a regexp.Compile of a constant pattern whose error the caller reads. The pattern is fixed, so the error is always nil, and MustCompile compiles the same pattern.

Adapted from go-critic's regexpMust.

```
func must() *regexp.Regexp {
    re, _ := regexp.Compile("^a+$")
    return re
}
```

## regexpraw

Reports a regexp pattern written as an interpreted string that doubles every backslash and holds at least one. A raw string literal keeps a backslash as written, so the rewrite drops one backslash from each pair.

Inspired by staticcheck's S1007.

```
var re = regexp.MustCompile("\\d+")
```

## regexpsimplify

Reports a regexp character class that has a shorter spelling. Both spellings are correct, so this is a house rule.

Inspired by go-critic's regexpSimplify.

```
var digits = regexp.MustCompile(`[0-9]`)
```

## resetvalue

Reports a read of the value returned by (\*time.Timer).Reset that decides whether to drain the timer's channel. A receive after Reset can take the new timer's value.

Adapted from staticcheck's SA1025.

```
func restart(t *time.Timer) {
    if !t.Reset(time.Second) {
        <-t.C
    }
}
```

## runerange

Reports a range over a string converted to a rune slice. Ranging over the string decodes one rune per iteration and allocates nothing, where the conversion builds a slice of every rune. A loop whose key is not \_ is left alone, because a string index is a byte offset and a rune slice index is a position.

Adapted from staticcheck's SA6003.

```
func f(s string) {
    for _, r := range []rune(s) {
        use(r)
    }
}
```

## seekwhence

Reports a Seek call with an io.Seek\* constant in the offset argument. Seek takes the offset first and the whence second.

Adapted from staticcheck's SA1013.

```
func toEnd(f *os.File) {
    f.Seek(io.SeekEnd, 0)
}
```

## selectempty

Reports a select statement with no cases. The statement blocks forever, and a reader cannot tell a deliberate wait from a select whose cases were dropped. The rule reports without a fix.

Adapted from staticcheck's SA5004, which reports an empty default branch in a for-select loop.

```
func wait() {
    select {}
}
```

## selectone

Reports a select statement with a single case clause and no default. The send or receive in the case blocks the same way without the select. A select whose case body holds a break that leaves the select is left alone.

Adapted from staticcheck's S1000.

```
func wait(ch <-chan int) {
    select {
    case <-ch:
    }
}
```

## selfassert

Reports a type assertion from a type to itself. The assertion yields the value it was given, so it does nothing. A nil value panics in the assertion, so removing it would change behavior, and the rule reports without a fix.

Inspired by go-critic's sloppyTypeAssert.

```
func f(v any) any {
    return v.(any)
}
```

## shellcommand

Reports a call to exec.Command or exec.CommandContext whose program name holds whitespace. The package does not run a shell, so the whole string is looked up as one file name and the call usually fails. A program path that holds a space runs, and the rule reports it too.

Adapted from staticcheck's SA1005.

```
func f() *exec.Cmd {
    return exec.Command("alpha --one")
}
```

## shift

Reports a shift by a count at least as large as the operand's width, which moves every bit out of the value.

Imported from the x/tools vet suite.

```
func f(n uint8) uint8 {
    return n << 8
}
```

## sigchanyzer

Reports a signal.Notify call on an unbuffered channel. The signal package sends without blocking, so a signal that arrives with no receiver is lost.

Imported from the x/tools vet suite.

```
func f(ch chan os.Signal) {
    signal.Notify(ch, os.Interrupt)
}
```

## singlefield

Reports a struct of one field that the type could carry instead. A pointer, interface, embedded, or tagged field is left alone. The field and the type fold into a defined type, and for an exported one that changes every importer.

```
type name struct {
    text string
}
```

## slenstr

Reports a string converted to a byte slice only to take its length. len(\[]byte(s)) and len(s) count the same bytes, and len(s) does not build the slice.

Inspired by go-critic's stringXbytes, which reports the reverse length check.

```
func size(s string) int {
    return len([]byte(s))
}
```

## sliceclear

Reports a loop that writes the zero value to every element of a slice. clear does the same in one call, from Go 1.21.

Adapted from go-critic's sliceClear.

```
func reset(xs []int) {
    for i := range xs {
        xs[i] = 0
    }
}
```

## sliceloop

Reports a loop that copies a source slice into another slice one element at a time. A counted loop may also copy a string into a byte slice.

Adapted from staticcheck's S1001.

```
func duplicate(dst, src []string) {
    for i := range src {
        dst[i] = src[i]
    }
}
```

## slicesbackward

Reports a loop that reads a slice from its last element to its first. slices.Backward yields the same index and element, from Go 1.23.

Imported from golang.org/x/tools' modernize suite.

```
func f(xs []int) {
    for i := len(xs) - 1; i >= 0; i-- {
        use(xs[i])
    }
}
```

## slicesclip

Reports a full slice expression that clips the capacity to the length. slices.Clip writes the same expression, from Go 1.21.

Imported from golang.org/x/tools' modernize suite.

```
func f(xs []int) []int {
    return xs[:len(xs):len(xs)]
}
```

## slicescontains

Reports a loop that searches a slice for one value. slices.Contains and slices.ContainsFunc state the search in one call, from Go 1.21.

Imported from golang.org/x/tools' modernize suite.

```
func f(xs []int, want int) bool {
    for _, x := range xs {
        if x == want {
            return true
        }
    }
    return false
}
```

## sliceshift

Reports a loop that shifts elements within one slice by an offset. copy allows the source and destination to overlap.

Adapted from staticcheck's S1018.

```
func f(xs []int) {
    for i := 0; i < len(xs)-1; i++ {
        xs[i] = xs[i+1]
    }
}
```

## slicessort

Reports a sort.Slice over an ordered basic type that compares the elements by index. slices.Sort orders the same slice, from Go 1.21.

Imported from golang.org/x/tools' modernize suite.

```
func f(xs []int) {
    sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
}
```

## slog

Reports a log/slog call whose key-value pairs are wrong: an argument in a key position that is neither a string nor a slog.Attr, or a final key with no value.

Imported from the x/tools vet suite.

```
func f() {
    slog.Warn("message", 11, 12)
}
```

## sloppyreassign

Reports an assignment in the init clause of an if, switch, or for whose variable nothing outside the statement uses. A short declaration gives the variable the narrower scope where it is read.

Adapted from go-critic's sloppyReassign.

```
func f(name string) error {
    var err error
    if err = fetch(name); err != nil {
        return err
    }
    return nil
}
```

## slowloris

Reports an http.Server literal with no timeout for reading headers. A client that sends its headers slowly holds the connection open.

Adapted from gosec's G112.

```
func serve() error {
    srv := &http.Server{Addr: ":8080"}
    return srv.ListenAndServe()
}
```

## sortmismatch

Reports a sort.Slice, sort.SliceStable, or sort.SliceIsSorted call whose less function does not name the slice the call reads. The call orders, or tests, the elements by values it reads from somewhere else.

Adapted from go-critic's sortSlice.

```
func f(rows []row, keys []string) {
    sort.Slice(rows, func(i, j int) bool { return keys[i] < keys[j] })
}
```

## sortnosort

Reports an assignment that converts a slice to a sort slice type and assigns it back. The converted value has a type whose Sort method orders the elements, and nothing calls it, so the assignment sorts nothing. The suggested fix sorts the slice, which changes behavior.

Adapted from staticcheck's SA4029.

```
func f(names []string) {
    names = sort.StringSlice(names)
}
```

## sortslice

Reports a sort.Slice call whose first argument is not a slice. The call takes an interface value, so the sort panics when it reaches the argument.

Imported from the x/tools vet suite.

```
func f(m map[string]int) {
    sort.Slice(m, func(i, j int) bool { return true })
}
```

## sortstrings

Reports sort.Strings, sort.Ints, sort.Float64s, or a sort.Slice whose less function compares the two elements at the indexes it was given, where the element type is ordered. A sort.Slice over an element type that can hold a NaN is left alone. slices.Sort, available since Go 1.21, orders the same slice.

Adapted from staticcheck's S1032, which stops at the element-typed call, and x/tools' modernize slicessort; it replaces the latter in the suite.

```
func f(names []string) {
    sort.Strings(names)
}
```

## sprintfquotedstring

Reports a format string that writes quotes around a %s by hand. A quote or a backslash in the value ends the quoted text early, where %q escapes the value and supplies the quotes.

Adapted from go-critic's sprintfQuotedString.

```
func f(s string) string {
    return fmt.Sprintf("\"%s\"", s)
}
```

## stditerators

Reports a loop that counts to a standard library type's count and reads each element through its index method. The type offers an iterator over the same elements, and a range writes the loop.

Imported from golang.org/x/tools' modernize suite.

```
func f(t reflect.Type) {
    for i := 0; i < t.NumField(); i++ {
        use(t.Field(i))
    }
}
```

## stdmethods

Reports a method whose name matches a method of a well-known interface and whose signature does not match it. The type then does not satisfy the interface it looks like it implements.

Imported from the x/tools vet suite.

```
type buffer struct{}

func (b buffer) WriteTo(w io.Writer) error { return nil }
```

## stdversion

Reports a reference to a standard library symbol that a later Go release than the file's added. The file's version is its module's go directive or the //go:build line at its top, and no file counts as older than Go 1.21.

Imported from the x/tools vet suite.

```
//go:build go1.21

package p

import "maps"

func keys(m map[string]int) {
    _ = maps.Keys(m)
}
```

## strcompare

Reports a comparison of a strings.Compare or bytes.Compare result with zero. The comparison operators express the same test, and strings.Compare documents them as clearer and faster. A bytes.Compare result is reported only against == and !=, since its ordering has no shorter form.

Inspired by go-critic's stringsCompare.

```
func f(a, b string) bool {
    return strings.Compare(a, b) == 0
}
```

## strconcat

Reports a strings.Join of two or three strings with a literal separator. A concatenation allocates once and needs no slice literal in between.

Inspired by go-critic's stringConcatSimplify.

```
func f(a, b string) string {
    return strings.Join([]string{a, b}, "-")
}
```

## strconvrange

Reports a strconv base, bit size, or format argument outside the documented values.

Adapted from staticcheck's SA1030.

```
func f(s string) {
    strconv.ParseInt(s, 1, 64)
}
```

## strictvar

Reports a declaration the suite would write another way, such as a zero value given a value, a non-zero value under var, adjacent declarations left ungrouped, a preamble spread over several blocks, or an immediate call where a var and an init function do. The rewrite keeps every value and comment. An example's published code keeps its var and const blocks, and every other rule applies there.

```
func f() int {
    x := 0
    return x
}
```

## stringintconv

Reports a conversion of an integer to a string. The conversion yields the UTF-8 encoding of the number as a code point, where the decimal digits are what the code usually means. A rune or a byte is left alone, since the conversion is the intended form for both.

Imported from the x/tools vet suite.

```
func f(n int) string {
    return string(n)
}
```

## stringofint

Reports a conversion of an integer to a string. The conversion yields a string of one rune, and the digits of the number are what the code usually means; the fix writes them, which changes the output. A byte and a rune are left alone, since the conversion is the intended form for both.

Adapted from x/tools' stringintconv and revive's string-of-int.

```
func f(n int) string {
    return string(n)
}
```

## stringsbuilder

Reports string concatenation with +=. strings.Builder accumulates the text without copying the string at every step.

Imported from golang.org/x/tools' modernize suite.

```
func f(xs []string) string {
    s := ""
    for _, x := range xs {
        s += x
    }
    return s
}
```

## stringscount

Reports a loop that counts the elements of a slice with a counter. Each pass adds one, so the counter equals len(slice) after the loop.

Adapted from the loop-to-call shape of staticcheck's S1011, with len as the call.

```
func count(xs []string) int {
    n := 0
    for range xs {
        n++
    }
    return n
}
```

## stringscut

Reports a search with strings.Index or strings.LastIndex and a slice at the result. strings.Cut returns the text either side of the separator and whether it was there, from Go 1.18.

Imported from golang.org/x/tools' modernize suite.

```
func f(s string) string {
    if i := strings.Index(s, "@"); i >= 0 {
        return s[i+1:]
    }
    return s
}
```

## stringscutprefix

Reports a HasPrefix test followed by a TrimPrefix of the same prefix. strings.CutPrefix returns the remainder and whether the prefix was there, from Go 1.20.

Imported from golang.org/x/tools' modernize suite.

```
func f(s string) string {
    if strings.HasPrefix(s, "/") {
        return strings.TrimPrefix(s, "/")
    }
    return s
}
```

## stringsseq

Reports a range over the Split or Fields function of strings or bytes. The Seq forms, added in Go 1.24, yield the substrings without allocating a slice.

Imported from golang.org/x/tools' modernize suite.

```
func f(s string) {
    for _, field := range strings.Fields(s) {
        use(field)
    }
}
```

## stringxbytes

Reports a conversion between a string and a byte slice that the call does not need: copy and append take a string, len and an empty test keep the count, and Write pairs with WriteString. The conversion copies the bytes.

Inspired by go-critic's stringXbytes and preferStringWriter.

```
func f(dst []byte, src string) int {
    return copy(dst, []byte(src))
}
```

## strreplaceall

Reports a strings.Replace call whose count is -1. ReplaceAll names the same operation without the count.

Adapted from staticcheck's QF1004.

```
func f(s string) string {
    return strings.Replace(s, "a", "b", -1)
}
```

## structconvert

Reports a struct literal that copies every field of another struct whose fields have the same names, order, types, tags, and embedded status. A named type or alias from another package is not reported.

Adapted from staticcheck's S1016, which ignores tags.

```
type first struct{ Kind string }
type second struct{ Kind string }

func convert(v first) second {
    return second{Kind: v.Kind}
}
```

## structtag

Reports a struct field tag that reflect.StructTag.Get cannot read, as one that is not a quoted key-value pair, and an unexported field tagged for encoding/json or encoding/xml, which those packages skip.

Imported from the x/tools vet suite.

```
type item struct {
    name string `json:"name"`
}
```

## structtagdup

Reports a struct tag that names the same key twice. reflect.StructTag.Lookup returns the first value for a key, so it does not read the later pair.

Adapted from staticcheck's SA5008.

```
type item struct {
    Name string `json:"name" json:"label"`
}
```

## switchtrue

Reports a switch whose tag is the literal true. A switch with no tag compares every case with true.

Inspired by go-critic's switchTrue.

```
func f(n int) int {
    switch true {
    case n > 0:
        return 1
    }
    return 0
}
```

## syncmaploaddelete

Reports a sync.Map load followed by a delete of the same key. The pair leaves a window where a value another goroutine stores is deleted unseen.

Adapted from go-critic's syncMapLoadAndDelete.

```
func forget(m *sync.Map, key string) {
    if v, ok := m.Load(key); ok {
        m.Delete(key)
        use(v)
    }
}
```

## testcmp

Reports a reflect.DeepEqual call in a test file. DeepEqual compares unexported fields and says only that two values differ, where a comparator such as go-cmp can say where.

```
func TestSame(t *testing.T) {
    if !reflect.DeepEqual(1, 1) {
        t.Error("differs")
    }
}
```

## testhelpers

Reports a test helper that reaches for process-global state where testing scopes one to the test: t.Context for context.Background and context.TODO, t.Setenv for os.Setenv, t.TempDir for os.MkdirTemp, and t.Chdir for os.Chdir.

```
func TestWrite(t *testing.T) {
    os.Setenv("KIND", "alpha")
}
```

## testingcontext

Reports a cancelable context created in a test and canceled by a deferred call. t.Context, added in Go 1.24, cancels the context when the test ends.

Imported from golang.org/x/tools' modernize suite.

```
func TestRun(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    use(ctx)
}
```

## testinggoroutine

Reports a call to Fatal, Fatalf, FailNow, Skip, Skipf, or SkipNow in a goroutine a test starts. Those methods stop the goroutine that calls them, and a goroutine a test starts is not the one running the test.

Imported from the x/tools vet suite.

```
func TestRun(t *testing.T) {
    go func() {
        t.Fatal("stopped")
    }()
}
```

## testlabel

Reports a subtest whose name is a constant string. A subtest is one case in a family, and a constant describes one thing.

```
func TestEvict(t *testing.T) {
    t.Run("removes the entry", func(t *testing.T) {})
}
```

## tests

Reports a test function whose name or signature is wrong, and an example that documents an identifier the package does not have. The functions checked are Test, Benchmark, Fuzz, and Example.

Imported from the x/tools vet suite.

```
func TestBad(n int) {}
```

## textblock

Reports a text block built a line at a time from string constants, where a raw string literal would hold the same text. The forms are a strings.Join of two or more string literals with a newline separator and a + chain of four or more string literals where every operand but the last ends with a newline. A block with a back quote in a line is left alone, because a raw literal cannot hold one.

```
func text() string {
    return strings.Join([]string{"alpha", "beta"}, "\n")
}
```

## tidytypes

Reports a parameter or result that writes a type the name before it already gave. One type covers a run of names.

```
func f(a int, b int) int { return a + b }
```

## timeafter

Reports a time.NewTimer used only through a deferred Stop and one receive in a select beside its declaration. time.After writes the same wait, and since Go 1.23 the collector reclaims the timer.

```
func f(ch chan int) error {
    timer := time.NewTimer(time.Second)
    defer timer.Stop()
    select {
    case <-timer.C:
        return errTimeout
    }
    return nil
}
```

## timedate

Reports a numeric date written in a format that does not sort: the year last, or a year-first date with a field that is not padded.

```
const date = "3/1/2024"
```

## timeequal

Reports a comparison of two time.Time values with == or !=, including a comparison against the zero-value literal. The operator compares the location and the monotonic clock reading as well; Equal compares the instant, and IsZero tests whether the time is zero.

Inspired by revive's time-equal.

```
func f(a, b time.Time) bool {
    return a == b
}
```

## timeformat

Reports a format that writes the year, the day, then the month. No common date standard runs year-day-month, so 2006-01-02 is usually what was meant.

Imported from the x/tools vet suite.

```
func f(t time.Time) string {
    return t.Format("2006-02-01")
}
```

## timelayout

Reports a time layout that names no reference element, prints the hour of a 12-hour clock with no PM or pm, or does not parse back. A layout is the reference time, Mon Jan 2 15:04:05 MST 2006, written the way the output should read.

Adapted from staticcheck's SA1002.

```
func f(t time.Time) string {
    return t.Format("yyyy-mm-dd")
}
```

## timenowloop

Reports a call to time.Now in the condition of a for statement. The condition is evaluated before each iteration, so a call there can run on each pass.

```
for time.Now().Before(deadline) {
    ready()
}
```

## timeuntil

Reports a Before or After comparison of time.Now().Add(d) with a time.Time. The same test is a comparison of time.Until(t) or time.Since(t) with a duration.

Adapted from staticcheck's S1024.

```
func f(deadline time.Time, wait time.Duration) bool {
    return time.Now().Add(wait).Before(deadline)
}
```

## todocomment

Reports a TODO, FIXME, or BUG doc comment with no detail. The note gives the next reader nothing to act on.

Inspired by go-critic's todoCommentWithoutDetail.

```
// TODO
func f() {}
```

## trimprefix

Reports a HasPrefix guard around a TrimPrefix whose result is assigned back to the same operand with the same prefix, in a file before Go 1.20. TrimPrefix returns its input unchanged when the prefix is absent, so the guard changes nothing. At Go 1.20 and later, stringscutprefix reports the pair.

Adapted from staticcheck's S1017, which rewrites a manual slice.

```
if strings.HasPrefix(s, prefix) {
    s = strings.TrimPrefix(s, prefix)
}
```

## truncatecmp

Reports a comparison through a conversion to a narrower integer type. The conversion keeps the low bits, so the comparison reads a truncated value.

Adapted from go-critic's truncateCmp.

```
func f(wide int64, bound int32) bool {
    return int32(wide) < bound
}
```

## typeassertchain

Reports an if-else chain of type assertions on one value. A type switch states the expression once and puts each type in a case.

Inspired by go-critic's typeAssertChain.

```
func f(x any) {
    if v, ok := x.(int); ok {
        print(v)
    } else if v, ok := x.(string); ok {
        print(v)
    }
}
```

## typedefirst

Reports a type declared after a method on it. A reader who opens the file to learn what the type holds meets a method first.

Adapted from go-critic's typeDefFirst.

```
func (s *session) touch() {
    s.count++
}

type session struct {
    count int
}
```

## typeswitchvar

Reports a type switch whose cases assert the value again. Binding the value in the switch header gives each case its typed value once.

Inspired by go-critic's typeSwitchVar.

```
func f(x any) int {
    switch x.(type) {
    case int:
        return x.(int)
    }
    return 0
}
```

## typeunparen

Reports parentheses around a type in a position gofmt leaves alone, as a var or type declaration or a map value. The type reads the same without them.

Inspired by go-critic's typeUnparen.

```
var x (int) = 1
```

## uncatchablesignal

Reports a signal.Notify or signal.NotifyContext call that names SIGKILL or SIGSTOP. Neither signal can be caught, so the registration changes nothing.

Adapted from staticcheck's SA1016.

```
func f(ch chan os.Signal) {
    signal.Notify(ch, syscall.SIGKILL)
}
```

## uncheckedinlineerr

Reports an empty body in an if that checks an error from its init. A comment on a line of its own in the body is left alone.

Inspired by go-critic's uncheckedInlineErr and staticcheck's SA9003.

```
func write(w io.Writer, src string) error {
    if _, err := io.WriteString(w, src); err != nil {
    }
    return nil
}
```

## uncondrecursion

Reports a call that runs whenever the function does and calls the function again, as an unguarded recursive call does. The stack grows until the program crashes.

Adapted from revive's unconditional-recursion.

```
func f() {
    f()
}
```

## unconvert

Reports a conversion to a type its operand already has, as int(n) for an int n or \[]byte(b) for a \[]byte b. A conversion to a floating-point or complex type is left alone, since an explicit one rounds and prevents a fused operation. The fix drops the conversion, adding parentheses where regrouping, a control clause, or an adjacent operator would change the value or the parse.

Adapted from mdempsky's unconvert.

```
func f(n int) int {
    return int(n)
}
```

## underef

Reports a dereference in front of a selector. The spec makes x.f shorthand for (\*x).f when f is a field, and both panic on a nil pointer.

Adapted from go-critic's underef.

```
func kind(p *item) string {
    return (*p).Kind
}
```

## unexportedreturn

Reports an exported function or method whose result is an unexported type declared in the same package, so a caller cannot write the type the result is declared with.

Adapted from revive's unexported-return.

```
func New() option {
    return option{}
}
```

## unlabelstmt

Reports a label that no goto names and that every break or continue reaching it would reach without the name. The label adds nothing.

Inspired by go-critic's unlabelStmt.

```
func f(xs []int) {
outer:
    for range xs {
        break outer
    }
}
```

## unlambda

Reports a function literal whose body only forwards its arguments to another function. Naming that function does the same with less code. A literal whose func type is wider than the function's is left alone.

Inspired by go-critic's unlambda.

```
func f() {
    use(func(s string) []string { return strings.Fields(s) })
}
```

## unmarshal

Reports a value passed to an unmarshal or decode function that is neither a pointer nor an interface. The function writes through the argument, so a value it cannot write through stays as it was.

Imported from the x/tools vet suite.

```
func f(b []byte) error {
    var v map[string]int
    return json.Unmarshal(b, v)
}
```

## unnecessaryblock

Reports a block statement that scopes nothing. A name's scope ends at the innermost block that contains it, so a block in a statement list that declares nothing adds a scope nothing uses. A clause is already a block, so a block that is the whole of a case or select clause is reported even when it declares a name.

Adapted from go-critic's unnecessaryBlock.

```
func f(n int) {
    {
        println(n)
    }
}
```

## unnecessarydefer

Reports a deferred call that ends the function, where a plain call runs at the same moment. The defer adds a keyword and nothing else.

Inspired by go-critic's unnecessaryDefer.

```
func f() {
    defer release()
}
```

## unnecessaryformat

Reports a fmt.Printf, fmt.Sprintf, fmt.Fprintf, or testing Errorf call whose format string holds no verb. The Print, Sprint, Fprint, and Error functions write the same text.

Inspired by revive's unnecessary-format.

```
func f() {
    fmt.Printf("done\n")
}
```

## unreachable

Reports a statement that execution cannot reach, as one below a return, a panic, or an endless loop.

Imported from the x/tools vet suite.

```
func f() int {
    return 1
    println("never")
}
```

## unreachablecase

Reports a type in a type switch case that an earlier interface case matches. Cases are tested in source order, and a case with an interface type matches every value whose type implements it, so the case for a later type that implements that interface never runs.

Adapted from staticcheck's SA4020.

```
func f(v any) {
    switch v.(type) {
    case io.Reader:
    case *bytes.Buffer:
    }
}
```

## unreachablereturn

Reports a return statement after a panic or an endless loop in the same block. Neither falls through, so a return after one does not run unless a goto reaches it.

Inspired by the x/tools unreachable pass.

```
func f() {
    panic("some message")
    return
}
```

## unsafefuncs

Reports pointer arithmetic written through uintptr conversions, as unsafe.Pointer(uintptr(p) + n). unsafe.Add and unsafe.Slice write the same arithmetic, and the garbage collector can follow them.

Imported from golang.org/x/tools' modernize suite.

```
func f(p unsafe.Pointer, n uintptr) unsafe.Pointer {
    return unsafe.Pointer(uintptr(p) + n)
}
```

## unsafeptr

Reports a conversion of a uintptr to unsafe.Pointer that leaves a stored uintptr holding an address. The garbage collector cannot see that word, and a stack copy moves what it points to.

Imported from the x/tools vet suite.

```
func f(u uintptr) *int {
    return (*int)(unsafe.Pointer(u))
}
```

## unsignedcmp

Reports a comparison of an unsigned value against a limit of its type. A value of type uint8 is never negative and never above 255, so a comparison against either limit is always false or always true.

Adapted from staticcheck's SA4003, which also reports the bounds of a signed type.

```
func below(n uint8) bool {
    return n < 0
}
```

## unslice

Reports a slice expression that takes the whole value, as s\[:]. The expression returns the value it was given.

Inspired by go-critic's unslice.

```
func f(s []int) []int {
    return s[:]
}
```

## unusedctxkey

Reports a context key constant with no use outside a Context.Value lookup.

```
type key int

const deadKey key = 0

func read(ctx context.Context) any {
    return ctx.Value(deadKey)
}
```

## unusedparams

Reports a parameter of a declared function that the body never reads. Renaming it to the blank identifier says the position is deliberate, and an exported function, a matching interface method, and a function used as a value keep their names.

A mirror of gopls' unusedparams, which is internal.

```
func unread(a int, b string) string {
    return b
}
```

## unusedrecv

Reports a method receiver whose name the body never reads. Go allows a receiver with no name, and a name the body ignores suggests the method works on the value.

Inspired by revive's unused-receiver.

```
type item struct{}

func (i *item) read() int {
    return 0
}
```

## unusedresult

Reports a call whose result is discarded, for the functions whose result is their only effect, as fmt.Errorf's is. The set is a fixed list of standard library functions, which the -unusedresult.funcs flag extends.

Imported from the x/tools vet suite.

```
func f() {
    fmt.Errorf("never used")
}
```

## unusedtypeparam

Reports a type parameter a function, method, or type alias never uses. A constraint does not use the parameter it constrains.

The rule is original to this suite.

```
func f[T any](n int) int { return n }
```

## unusedvariable

Reports a variable that is declared and never read. Go will not compile the package, so the fix is what the analysis is for: it removes the declaration and every write to the variable, keeping a value when evaluating it has an effect.

A mirror of gopls' unusedvariable, which is internal.

```
func f() {
    v := "a"
}
```

## unusedwrite

Reports a write to a field or an element of a value that was copied, as the loop variable of a range over a slice of structs. The write reaches the copy and nothing reads it.

Imported from the x/tools vet suite.

```
func f(rows []item) {
    for _, row := range rows {
        row.count = 1
    }
}
```

## unusefultag

Reports a json struct tag that changes nothing: one on an unexported field, or one with no name and no options.

Adapted from x/tools' structtag; inspired by staticcheck's SA9005.

```
type row struct {
    name string `json:"name"`
}
```

## unwrap

Reports a construct written across several lines that fits on one: a call, a binary expression, a header, or a block of one spec.

A function signature goes back on one line whatever its width, because a signature is never broken across lines. That covers a function type in a field, a map value, or a parameter.

```
func f() {
    use(1,
        2)
}
```

## urlconst

Reports a call to url.Parse with a constant URL that it rejects. The call returns the same error wherever it runs.

Inspired by staticcheck's SA1007.

```
func f() {
    url.Parse("http://[::1")
}
```

## uselessbreak

Reports a break at the end of a switch case. The switch ends after the case either way, so the break has no effect; a break at the end of a loop body ends the loop, so it is not reported.

Adapted from revive's useless-break, which also checks select clauses.

```
func f(n int) {
    switch n {
    case 0:
        break
    }
}
```

## uselessconcat

Reports string literals joined with + where the joined literal fits on the line.

```
var msg = "one " + "two"
```

## uuid

Reports a UUID built by formatting hex digits into the shape of one. A UUID has one textual form and one parser, and both live in a library.

```
func f(b []byte) string {
    return fmt.Sprintf("%x-%x-%x-%x-%x",
        b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
```

## valswap

Reports a swap written through a temporary variable. Go evaluates every right-hand side before it assigns any, so x, y = y, x needs no temporary.

Inspired by go-critic's valSwap.

```
func f(x, y int) (int, int) {
    tmp := x
    x = y
    y = tmp
    return x, y
}
```

## valuereceiver

Reports a write to a field of a value receiver that nothing reads afterwards. The method works on a copy, so the write is lost.

Inspired by revive's modifies-value-receiver.

```
func (p pair) set(left int) {
    p.left = left
}
```

## vargrouptype

Reports an untyped constant whose default type is not the one a spec above it declares, where that type could hold the value. A const block passes a type down only when the spec below also omits its expression list, and a variable declared without a type takes the type of its value.

Adapted from staticcheck's SA9004.

```
const (
    first  byte = 1
    second      = 2
)
```

## waitgroup

Reports a sync.WaitGroup.Add call made inside a new goroutine, which can run after the Wait that waits for it.

Imported from the x/tools vet suite.

```
func f(wg *sync.WaitGroup) {
    go func() {
        wg.Add(1)
        defer wg.Done()
    }()
    wg.Wait()
}
```

## waitgroupgo

Reports an Add and the go statement that pairs with it through a deferred Done. wg.Go, added in Go 1.25, starts the goroutine and keeps the count.

Imported from golang.org/x/tools' modernize suite.

```
func f(wg *sync.WaitGroup) {
    wg.Add(1)
    go func() {
        defer wg.Done()
        work()
    }()
}
```

## weakrsa

Reports a call to rsa.GenerateKey with a constant key size below 2048 bits, the minimum NIST SP 800-57 recommends through 2030. By default, rsa.GenerateKey returns an error for a key below 1024 bits.

Adapted from gosec's G403.

```
func generate() {
    rsa.GenerateKey(rand.Reader, 1024)
}
```

## weaktls

Reports a tls.Config literal with InsecureSkipVerify set to the constant true, or with a nonzero MinVersion constant below TLS 1.2. When InsecureSkipVerify is true, crypto/tls accepts any certificate presented by the server and any host name in that certificate. The zero MinVersion selects the TLS 1.2 default.

Adapted from gosec's G402.

```
func skip() *tls.Config {
    return &tls.Config{InsecureSkipVerify: true}
}
```

## wgbody

Reports a Done call on the group a sync.WaitGroup.Go call starts, unless an earlier Add that is neither deferred nor started with go can raise the count. Reports an unrecovered panic in the function passed to Go. The Done call removes the task Go added, and the sync documentation says that function must not panic.

Inspired by revive's forbidden-call-in-wg-go.

```
func start(wg *sync.WaitGroup) {
    wg.Go(func() {
        wg.Done()
    })
}
```

## wgogoroutine

Reports an Add or a Go call that raises a sync.WaitGroup's count from inside a goroutine instead of before it starts. A Wait can return before the count is raised.

Adapted from the x/tools vet pass waitgroup and staticcheck's SA2000.

```
func f(wg *sync.WaitGroup) {
    go func() {
        wg.Add(1)
        work()
    }()
    wg.Wait()
}
```

## whitespace

Reports a blank line in the wrong place around declarations: one is needed between a multi-line top-level declaration and its neighbors, and before a doc comment inside a block or a struct field list.

```
func b() {
    println(1)
}
func c() {}
```

## wrapperarg

Reports a call that passes an argument a convenience wrapper already supplies. The argument is a count, a delta, a length, a nil mask, or a mapping.

Adapted from go-critic's wrapperFunc. The wrapperfunc analyzer uses that name for a different rule.

```
func splitEvery(s, sep string) []string {
    return strings.SplitN(s, sep, -1)
}
```

## wrapperfunc

Reports an unexported function or method with no doc comment whose body only forwards its parameters to another function of the same signature. A wrapper another package can call, or one the doc comment explains, is left alone. A second name for the same behavior is one more name to choose at every call.

The name comes from go-critic's wrapperFunc.

```
func outer(x int) int {
    return inner(x)
}
```

## writebytes

Reports a byte slice converted to a string for io.WriteString when the writer is concrete and does not implement io.StringWriter. Write takes the byte slice, so the conversion is unnecessary.

Adapted from staticcheck's SA6006.

```
type sink struct{}

func (sink) Write([]byte) (int, error) { return 0, nil }

func write(w sink, b []byte) {
    io.WriteString(w, string(b))
}
```

## yield

Reports a call to a yield function that may be reached after an earlier call returned false, in a package that imports iter. A range loop ends when yield returns false, so the iterator must not call it again.

Adapted from gopls' yield, which is internal.

```
func sequence(yield func(int) bool) {
    yield(0)
    yield(1)
}
```

## yoda

Reports an == or != comparison with a literal or nil on the left. Go code reads left to right as a sentence about the variable, and a mistyped assignment does not compile in Go.

Inspired by go-critic's yodaStyleExpr.

```
func f(err error) bool {
    return nil == err
}
```

## zerorepeat

Reports a run of zero bytes built with bytes.Repeat. make allocates the same result and leaves it zeroed.

Inspired by go-critic's zeroByteRepeat.

```
func f(n int) []byte {
    return bytes.Repeat([]byte{0}, n)
}
```
