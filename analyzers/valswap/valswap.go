// Package valswap provides an analysis.Analyzer that reports a swap written
// through a temporary.
//
// Go evaluates every right-hand side before it assigns any of them, so
// x, y = y, x swaps two values with no temporary. The temporary came from
// languages that assign one variable at a time.
package valswap

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "valswap",
	Doc:  "reports a swap written through a temporary",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			block, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			check(pass, block)
			return true
		})
	}
	return nil, nil
}

// check reports each three-statement swap in block.
func check(pass *analysis.Pass, block *ast.BlockStmt) {
	for i := 0; i+2 < len(block.List); i++ {
		tmp, x, y, ok := swap(block.List[i], block.List[i+1], block.List[i+2])
		if !ok || used(pass, block.List[i+3:], tmp) {
			continue
		}
		pass.Reportf(block.List[i].Pos(),
			"swap through %s; write %s, %s = %s, %s instead",
			tmp, x, y, y, x,
		)
	}
}

// swap returns the names of a three-statement swap, if the statements are one.
func swap(first, second, third ast.Stmt) (tmp, x, y string, ok bool) {
	tmpStmt, ok := single(first, token.DEFINE)
	if !ok {
		return "", "", "", false
	}
	tmpID, ok := tmpStmt.Lhs[0].(*ast.Ident)
	if !ok {
		return "", "", "", false
	}
	aID, ok := tmpStmt.Rhs[0].(*ast.Ident)
	if !ok {
		return "", "", "", false
	}
	// a = b, then b = tmp.
	_, bID, ok := assignTo(second, aID.Name)
	if !ok || bID.Name == aID.Name {
		return "", "", "", false
	}
	if _, rhs, ok := assignTo(third, bID.Name); !ok || rhs.Name != tmpID.Name {
		return "", "", "", false
	}
	return tmpID.Name, aID.Name, bID.Name, true
}

// assignTo returns the two sides of an assignment to want, if stmt is one.
func assignTo(stmt ast.Stmt, want string) (lhs, rhs *ast.Ident, ok bool) {
	assign, ok := single(stmt, token.ASSIGN)
	if !ok {
		return nil, nil, false
	}
	lhs, ok = assign.Lhs[0].(*ast.Ident)
	if !ok || lhs.Name != want {
		return nil, nil, false
	}
	rhs, ok = assign.Rhs[0].(*ast.Ident)
	return lhs, rhs, ok
}

// single returns stmt as an assignment of one value to one name.
func single(stmt ast.Stmt, tok token.Token) (*ast.AssignStmt, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != tok {
		return nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	return assign, true
}

// used reports whether name is read anywhere in stmts.
func used(pass *analysis.Pass, stmts []ast.Stmt, name string) bool {
	var (
		found bool
		visit = func(n ast.Node) bool {
			if found {
				return false
			}
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			id, ok := n.(*ast.Ident)
			if !ok || id.Name != name {
				return true
			}
			if pass.TypesInfo.Uses[id] != nil {
				found = true
				return false
			}
			return true
		}
	)
	for _, stmt := range stmts {
		ast.Inspect(stmt, visit)
	}
	return found
}
