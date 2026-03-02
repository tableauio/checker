package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"slices"
)

type ASTKey struct {
	TypeName string
	FuncName string
}

func parseAST(file *ast.File) map[ASTKey]bool {
	astMap := map[ASTKey]bool{}
	for _, decl := range file.Decls {
		switch typ := decl.(type) {
		case *ast.GenDecl:
			if typ.Tok != token.TYPE {
				continue
			}
			for _, spec := range typ.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				// mark type as exists
				astMap[ASTKey{
					TypeName: typeSpec.Name.Name,
				}] = true
			}
		case *ast.FuncDecl:
			// check function declarations
			if typ.Recv == nil {
				continue
			}
			// check receiver typename
			if len(typ.Recv.List) != 1 {
				continue
			}
			// must be pointer receiver
			recvType, ok := typ.Recv.List[0].Type.(*ast.StarExpr)
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
				FuncName: typ.Name.Name,
			}] = true
		}
	}
	return astMap
}

func removeInitFuncAndTrailingNotes(file *ast.File, fset *token.FileSet) string {
	type rangeToRemove struct {
		start token.Pos
		end   token.Pos
	}
	var removedRanges []*rangeToRemove
	// remove init functions
	file.Decls = slices.DeleteFunc(file.Decls, func(decl ast.Decl) bool {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			return false
		}
		if funcDecl.Name.Name != "init" || funcDecl.Recv != nil {
			return false
		}
		start := funcDecl.Pos()
		if funcDecl.Doc != nil {
			start = funcDecl.Doc.Pos()
		}
		end := funcDecl.End()
		removedRanges = append(removedRanges, &rangeToRemove{start, end})
		return true
	})
	// remove init function notes and trailing notes
	file.Comments = slices.DeleteFunc(file.Comments, func(cg *ast.CommentGroup) bool {
		return cg.End() < file.Package || slices.ContainsFunc(removedRanges, func(r *rangeToRemove) bool {
			return cg.Pos() >= r.start && cg.Pos() < r.end
		})
	})
	buf := new(bytes.Buffer)
	err := format.Node(buf, fset, file)
	if err != nil {
		panic(err)
	}
	return buf.String()
}
