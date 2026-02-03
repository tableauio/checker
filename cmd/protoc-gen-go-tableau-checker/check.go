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
		filename := filepath.Join(f.GeneratedFilenamePrefix + "." + checkExt + ".go")
		path := filepath.Join(params.outdir, filename)
		existed, err := Exists(path)
		if err != nil {
			panic(err)
		}
		g := gen.NewGeneratedFile(filename, "")
		if existed {
			addIncrementalFileContent(f, g, path)
		} else {
			generateFileHeader(f, g)
			g.P()
			g.P("package ", params.pkg)
			g.P("import (")
			g.P("tableau ", loaderImportPath)
			g.P(")")
			g.P()
			generateFileContent(f, g)
		}
	}
}

func addIncrementalFileContent(file *protogen.File, g *protogen.GeneratedFile, path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	g.P(string(content))
	fset := token.NewFileSet()
	ast, err := parser.ParseFile(fset, path, content, 0)
	if err != nil {
		panic(err)
	}
	astMap := parseAST(ast)
	for _, message := range file.Messages {
		opts := message.Desc.Options().(*descriptorpb.MessageOptions)
		worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
		if worksheet != nil {
			messagerName := string(message.Desc.Name())

			if _, ok := astMap[ASTKey{
				TypeName: messagerName,
				FuncName: "Check",
			}]; !ok {
				generateCheck(g, messagerName)
			}

			if _, ok := astMap[ASTKey{
				TypeName: messagerName,
				FuncName: "CheckCompatibility",
			}]; !ok {
				generateCheckCompatibility(g, messagerName)
			}
		}
	}
}

// generateFileContent generates struct type definitions.
func generateFileContent(file *protogen.File, g *protogen.GeneratedFile) {
	for _, message := range file.Messages {
		opts := message.Desc.Options().(*descriptorpb.MessageOptions)
		worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
		if worksheet != nil {
			messagerName := string(message.Desc.Name())
			generateCheck(g, messagerName)
			generateCheckCompatibility(g, messagerName)
		}
	}
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
