package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// generateMessager generates a protoconf file correponsing to the protobuf file.
// Each wrapped struct type implement the Messager interface.
func generateMessager(gen *protogen.Plugin) {
	for _, f := range gen.Files {
		if !NeedGenFile(f) {
			continue
		}
		var fileMessagers []string
		for _, message := range f.Messages {
			opts := message.Desc.Options().(*descriptorpb.MessageOptions)
			worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
			if worksheet != nil {
				messagerName := string(message.Desc.Name())
				fileMessagers = append(fileMessagers, messagerName)
			}
		}
		filename := filepath.Join(f.GeneratedFilenamePrefix + "." + checkExt + ".go")
		path := filepath.Join(params.outdir, filename)
		existed, err := Exists(path)
		if err != nil {
			panic(err)
		}
		g := gen.NewGeneratedFile(filename, "")
		generateFileHeader(gen, f, g, false)
		g.P()
		if existed {
			addIncrementalFileContent(g, fileMessagers, path)
		} else {
			g.P("package ", params.pkg)
			g.P("import (")
			g.P("tableau ", loaderImportPath)
			g.P(")")
			g.P()
			generateFileContent(g, fileMessagers)
		}
		generateRegister(g, fileMessagers)
	}
}

func addIncrementalFileContent(g *protogen.GeneratedFile, messagers []string, path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	fset := token.NewFileSet()
	ast, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	astMap := parseAST(ast)
	g.P(removeInitFuncAndTrailingNotes(ast, fset))
	for _, messager := range messagers {
		if _, ok := astMap[ASTKey{
			TypeName: messager,
		}]; !ok {
			generateTypeDecl(g, messager)
		}

		if _, ok := astMap[ASTKey{
			TypeName: messager,
			FuncName: "Check",
		}]; !ok {
			generateCheck(g, messager)
		}

		if _, ok := astMap[ASTKey{
			TypeName: messager,
			FuncName: "CheckCompatibility",
		}]; !ok {
			generateCheckCompatibility(g, messager)
		}
	}
}

// generateFileContent generates struct type definitions.
func generateFileContent(g *protogen.GeneratedFile, messagers []string) {
	for _, messager := range messagers {
		generateTypeDecl(g, messager)
		generateCheck(g, messager)
		generateCheckCompatibility(g, messager)
	}
}

func generateTypeDecl(g *protogen.GeneratedFile, messagerName string) {
	g.P("type ", messagerName, " struct {")
	g.P("tableau.", messagerName)
	g.P("}")
	g.P()
}

func generateCheck(g *protogen.GeneratedFile, messagerName string) {
	g.P("func (x *", messagerName, ") Check(hub *tableau.Hub) error {")
	g.P("// TODO: implement here.")
	g.P("return nil")
	g.P("}")
	g.P()
}

func generateCheckCompatibility(g *protogen.GeneratedFile, messagerName string) {
	g.P("func (x *", messagerName, ") CheckCompatibility(hub, newHub *tableau.Hub) error {")
	g.P("// TODO: implement here.")
	g.P("return nil")
	g.P("}")
	g.P()
}

func generateRegister(g *protogen.GeneratedFile, messagers []string) {
	g.P("func init() {")
	for _, messager := range messagers {
		g.P("register(func() checker { return new(", messager, ") })")
	}
	g.P("}")
}
