// Package weaktls provides an analysis.Analyzer that reports a tls.Config
// literal that disables certificate verification or permits TLS below 1.2.
//
// The crypto/tls documentation for Config.InsecureSkipVerify says that when
// the field is true, crypto/tls accepts any certificate presented by the
// server and any host name in that certificate, and that TLS is then
// susceptible to machine-in-the-middle attacks unless custom verification is
// used. The documentation for Config.MinVersion says the zero value uses TLS
// 1.2 as the minimum. InsecureSkipVerify is a client setting; the rule reports
// it in any tls.Config literal, because the literal does not show whether it
// configures a client or a server.
//
// A literal that sets VerifyConnection to an expression other than nil
// suppresses the InsecureSkipVerify report: the documentation names custom
// verification as a reason to set the field. The rule does not read the
// callback, and a callback held in a variable counts. A literal that sets
// VerifyPeerCertificate to an expression other than nil suppresses the report
// only when it also sets SessionTicketsDisabled to the constant true, because
// that callback is not invoked on a resumed connection. A _test.go file is
// left alone: the rule skips the whole file, because a test config is not part
// of the built program and the documentation says InsecureSkipVerify is for
// testing. A non-constant InsecureSkipVerify or MinVersion is left alone,
// because its value is not visible to the tool. A nonzero MinVersion below
// TLS 1.2 is reported; the zero value selects the default.
//
// Report only: removing the field or raising the version makes a connection
// to an unverified server or to an older protocol fail, and the author's
// intent is not visible to the tool.
//
// The rule is adapted from gosec's G402. gosec checks cipher suites,
// MaxVersion, and PreferServerCipherSuites too, reports an InsecureSkipVerify
// value it cannot resolve, and reads an assignment as well as a composite
// literal; weaktls reads the constant value with go/types, reports only the
// two fields, and reads literals only. The README catalogs every check and
// exemption.
package weaktls

import (
	"crypto/tls"
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "weaktls",
	Doc:  "reports a tls.Config literal that disables certificate verification or permits TLS below 1.2",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.Pos()).Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if lit, ok := n.(*ast.CompositeLit); ok && isConfig(pass, lit) {
				check(pass, lit)
			}
			return true
		})
	}
	return nil, nil
}

// check reports the two insecure fields of a tls.Config literal.
func check(pass *analysis.Pass, lit *ast.CompositeLit) {
	custom := customVerification(pass, lit)
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "InsecureSkipVerify":
			if !custom && truth(pass, kv.Value) {
				pass.Reportf(
					kv.Value.Pos(),
					"InsecureSkipVerify accepts any certificate and host name",
				)
			}
		case "MinVersion":
			if old(pass, kv.Value) {
				pass.Reportf(
					kv.Value.Pos(),
					"MinVersion below the TLS 1.2 default",
				)
			}
		}
	}
}

// customVerification reports whether lit sets a callback that runs on every
// connection: VerifyConnection, or VerifyPeerCertificate with resumption
// disabled. The rule does not read the callback, and any expression other than
// nil counts.
func customVerification(pass *analysis.Pass, lit *ast.CompositeLit) bool {
	var connection, peer, noResume bool
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "VerifyConnection":
			connection = !nilValue(pass, kv.Value)
		case "VerifyPeerCertificate":
			peer = !nilValue(pass, kv.Value)
		case "SessionTicketsDisabled":
			noResume = truth(pass, kv.Value)
		}
	}
	return connection || (peer && noResume)
}

// truth reports whether expr is the constant true.
func truth(pass *analysis.Pass, expr ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[expr]
	return ok && tv.Value != nil && tv.Value.Kind() == constant.Bool &&
		constant.BoolVal(tv.Value)
}

// old reports whether expr is a nonzero constant below TLS 1.2. A zero value
// selects the default minimum, which is TLS 1.2; any other value below TLS 1.2
// sets the minimum lower and permits TLS 1.0 or 1.1.
func old(pass *analysis.Pass, expr ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[expr]
	if !ok || tv.Value == nil {
		return false
	}
	version, ok := constant.Uint64Val(constant.ToInt(tv.Value))
	return ok && version != 0 && version < tls.VersionTLS12
}

// nilValue reports whether expr is nil, including a conversion of nil to the
// callback's type.
func nilValue(pass *analysis.Pass, expr ast.Expr) bool {
	expr = ast.Unparen(expr)
	if tv, ok := pass.TypesInfo.Types[expr]; ok && tv.IsNil() {
		return true
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	if tv, ok := pass.TypesInfo.Types[call.Fun]; !ok || !tv.IsType() {
		return false
	}
	return nilValue(pass, call.Args[0])
}

// isConfig reports whether lit has the type crypto/tls.Config, or a pointer to
// it.
func isConfig(pass *analysis.Pass, lit *ast.CompositeLit) bool {
	t := pass.TypesInfo.TypeOf(lit)
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if pointer, ok := t.Underlying().(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "crypto/tls" &&
		obj.Name() == "Config"
}
