package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

var (
	loaderImportPath    protogen.GoImportPath
	loaderImportPathSet bool
)

// setLoaderImportPath records the loader import path used by generated hub code.
// All workbook files in one generation must share the same loader import path.
func setLoaderImportPath(path protogen.GoImportPath) error {
	if loaderImportPathSet && loaderImportPath != path {
		return fmt.Errorf("inconsistent loader import path: got %q, want %q", path, loaderImportPath)
	}
	loaderImportPath = path
	loaderImportPathSet = true
	return nil
}

// generateMessager generates a protoconf file corresponding to the protobuf file.
// Each wrapped struct type implement the Messager interface.
func generateMessager(gen *protogen.Plugin, file *protogen.File) error {
	path := protogen.GoImportPath(string(file.GoImportPath) + "/" + params.loaderPkg)
	if err := setLoaderImportPath(path); err != nil {
		return err
	}
	// parse file messagers
	var fileMessagers []string
	for _, message := range file.Messages {
		opts := message.Desc.Options().(*descriptorpb.MessageOptions)
		worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
		if worksheet != nil {
			messagerName := string(message.Desc.Name())
			fileMessagers = append(fileMessagers, messagerName)
		}
	}
	// generate file
	filename := filepath.Join(file.GeneratedFilenamePrefix + "." + checkExt + ".go")
	outPath := filepath.Join(params.outdir, filename)
	exists, err := Exists(outPath)
	if err != nil {
		return fmt.Errorf("stat checker file %s: %w", outPath, err)
	}
	g := gen.NewGeneratedFile(filename, "")
	generateFileHeader(gen, file, g, false)
	g.P()
	if exists {
		if err := addIncrementalFileContent(g, fileMessagers, outPath); err != nil {
			return err
		}
	} else {
		g.P("package ", params.pkg)
		g.P("import (")
		g.P("tableau ", path)
		g.P(")")
		g.P()
		generateFileContent(g, fileMessagers)
	}
	generateRegister(g, fileMessagers)
	return nil
}

func addIncrementalFileContent(g *protogen.GeneratedFile, messagers []string, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read existing checker file %s: %w", path, err)
	}
	fset := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse existing checker file %s: %w", path, err)
	}
	astMap := parseAST(fileAST)
	body, err := removeInitFuncAndTrailingNotes(fileAST, fset)
	if err != nil {
		return fmt.Errorf("format existing checker file %s: %w", path, err)
	}
	g.P(body)
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
	return nil
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
	g.P("//nolint:gochecknoinits")
	g.P("func init() {")
	for _, messager := range messagers {
		g.P("register(func() checker { return new(", messager, ") })")
	}
	g.P("}")
}
