package main

import "go/ast"

type ASTKey struct {
	TypeName string
	FuncName string
}

func parseAST(file *ast.File) map[ASTKey]bool {
	astMap := map[ASTKey]bool{}
	for _, decl := range file.Decls {
		// check function declarations
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		// check receiver typename
		if len(fn.Recv.List) != 1 {
			continue
		}
		// must be pointer receiver
		recvType, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		ident, ok := recvType.X.(*ast.Ident)
		if !ok {
			continue
		}
		// mark function as exists
		astMap[ASTKey{
			TypeName: ident.Name,
			FuncName: fn.Name.Name,
		}] = true
	}
	return astMap
}
