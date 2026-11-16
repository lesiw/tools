// Package hostportconst provides an analysis.Analyzer that reports a constant
// address that a listen function cannot parse as host:port.
//
// For the tcp and udp networks, the address has the form host:port. The host,
// the port, or both can be empty, and a literal IPv6 host is enclosed in
// brackets. net.SplitHostPort parses those forms, and the port is a number
// from 0 to 65535 or a service name. An address that fails either check cannot
// work, and a constant is the same on every run.
//
// The functions checked are net.Listen, net.ListenPacket,
// (*net.ListenConfig).Listen, (*net.ListenConfig).ListenPacket,
// http.ListenAndServe and http.ListenAndServeTLS. Only a direct call is
// checked; a call through a function value is left alone. For a unix or IP
// network, the address has another form, so the call is left alone; so is a
// call whose network is not a constant string. The host is not resolved, and a
// service name is not looked up: an address that parses but names a host or
// service that does not exist is not reported.
//
// Report only: the intended address is not known to the tool.
//
// The rule is adapted from staticcheck's SA1020, which checks only the
// net/http functions.
package hostportconst

import (
	"go/ast"
	"go/constant"
	"go/types"
	"net"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "hostportconst",
	Doc:  "reports a constant listen address that does not parse as host:port",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

// target holds the argument positions of a listen function's network and
// address. A negative network index means the call always uses tcp.
type target struct {
	network int
	address int
}

func check(pass *analysis.Pass, call *ast.CallExpr) {
	tgt, ok := callTarget(pass, call)
	if !ok {
		return
	}
	if tgt.network >= 0 && !internetNetwork(pass, call, tgt.network) {
		return
	}
	if tgt.address >= len(call.Args) {
		return
	}
	arg := call.Args[tgt.address]
	tv, ok := pass.TypesInfo.Types[arg]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return
	}
	address := constant.StringVal(tv.Value)
	if address == "" || valid(address) {
		return
	}
	pass.Reportf(arg.Pos(), "address is not a valid host:port")
}

// callTarget returns the target of a call to one of the listen functions, and
// false for any other call.
func callTarget(pass *analysis.Pass, call *ast.CallExpr) (target, bool) {
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		return selectorTarget(pass, fun)
	case *ast.Ident:
		return identTarget(pass, fun)
	}
	return target{}, false
}

// selectorTarget returns the target of a call written as pkg.F or x.F.
func selectorTarget(pass *analysis.Pass, fun *ast.SelectorExpr) (target, bool) {
	obj, ok := pass.TypesInfo.Uses[fun.Sel].(*types.Func)
	if !ok || obj.Pkg() == nil {
		return target{}, false
	}
	switch obj.Pkg().Path() {
	case "net/http":
		return httpTarget(obj)
	case "net":
		return netTarget(pass, fun, obj)
	}
	return target{}, false
}

// identTarget returns the target of a call written as F, which a dot import
// allows.
func identTarget(pass *analysis.Pass, fun *ast.Ident) (target, bool) {
	obj, ok := pass.TypesInfo.Uses[fun].(*types.Func)
	if !ok || obj.Pkg() == nil {
		return target{}, false
	}
	switch obj.Pkg().Path() {
	case "net":
		return freeTarget(obj)
	case "net/http":
		return httpTarget(obj)
	}
	return target{}, false
}

// httpTarget returns the target of one of net/http's listen functions.
func httpTarget(obj *types.Func) (target, bool) {
	if !isFunction(obj) {
		return target{}, false
	}
	switch obj.Name() {
	case "ListenAndServe", "ListenAndServeTLS":
		return target{network: -1, address: 0}, true
	}
	return target{}, false
}

// netTarget returns the target of one of net's listen functions or methods.
func netTarget(pass *analysis.Pass, fun *ast.SelectorExpr, obj *types.Func) (target, bool) {
	if isFunction(obj) {
		return freeTarget(obj)
	}
	if !isListenConfig(obj) {
		return target{}, false
	}
	switch obj.Name() {
	case "Listen", "ListenPacket":
		sel := pass.TypesInfo.Selections[fun]
		if sel != nil && sel.Kind() == types.MethodExpr {
			return target{network: 2, address: 3}, true
		}
		return target{network: 1, address: 2}, true
	}
	return target{}, false
}

// freeTarget returns the target of net.Listen or net.ListenPacket.
func freeTarget(obj *types.Func) (target, bool) {
	if !isFunction(obj) {
		return target{}, false
	}
	switch obj.Name() {
	case "Listen", "ListenPacket":
		return target{network: 0, address: 1}, true
	}
	return target{}, false
}

// isFunction reports whether fn is a package function rather than a method.
func isFunction(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() == nil
}

// isListenConfig reports whether fn is a method of net.ListenConfig.
func isListenConfig(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	t := sig.Recv().Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "net" &&
		named.Obj().Name() == "ListenConfig"
}

// internetNetwork reports whether the network argument at index is one where
// the address has the form host:port.
func internetNetwork(pass *analysis.Pass, call *ast.CallExpr, index int) bool {
	if index >= len(call.Args) {
		return false
	}
	tv, ok := pass.TypesInfo.Types[call.Args[index]]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return false
	}
	switch network := constant.StringVal(tv.Value); network {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		return true
	}
	return false
}

// valid reports whether address parses as a host:port pair with a usable port.
func valid(address string) bool {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	return validPort(port)
}

// validPort reports whether port is empty, a number from 0 to 65535, or a
// service name. A service name is not looked up, because the service table
// differs by system.
func validPort(port string) bool {
	if port == "" {
		return true
	}
	n, err := strconv.ParseInt(port, 10, 64)
	if err != nil {
		return !numeric(port)
	}
	return n >= 0 && n <= 65535
}

// numeric reports whether port is an optional sign followed by digits.
func numeric(port string) bool {
	if port == "" {
		return false
	}
	if port[0] == '+' || port[0] == '-' {
		port = port[1:]
	}
	if port == "" {
		return false
	}
	for i := 0; i < len(port); i++ {
		if port[i] < '0' || port[i] > '9' {
			return false
		}
	}
	return true
}
