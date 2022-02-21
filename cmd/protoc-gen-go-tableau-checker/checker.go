package main

import (
	"path/filepath"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	errorsPackage = protogen.GoImportPath("errors")
	fmtPackage    = protogen.GoImportPath("fmt")
	formatPackage = protogen.GoImportPath("github.com/tableauio/tableau/format")
	loadPackage   = protogen.GoImportPath("github.com/tableauio/tableau/load")
)

// golbal container for record all proto filenames and messager names
var messagers []string
var loaderImportPath protogen.GoImportPath

// generateMessager generates a protoconf file correponsing to the protobuf file.
// Each wrapped struct type implement the Messager interface.
func generateMessager(gen *protogen.Plugin, file *protogen.File) {
	loaderImportPath = protogen.GoImportPath(string(file.GoImportPath) + "/" + *loaderPkg)

	filename := filepath.Join(*pkg, file.GeneratedFilenamePrefix+"."+pcExt+".go")
	g := gen.NewGeneratedFile(filename, "")
	generateFileHeader(gen, file, g)
	g.P()
	g.P("package ", *pkg)
	g.P()
	generateFileContent(gen, file, g)
}

// generateFileContent generates struct type definitions.
func generateFileContent(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile) {
	var fileMessagers []string
	for _, message := range file.Messages {
		opts := message.Desc.Options().(*descriptorpb.MessageOptions)
		worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
		if worksheet != nil {
			genMessage(gen, file, g, message)

			messagerName := string(message.Desc.Name())
			fileMessagers = append(fileMessagers, messagerName)
		}
	}
	messagers = append(messagers, fileMessagers...)
	generateRegister(fileMessagers, g)
}

func generateRegister(messagers []string, g *protogen.GeneratedFile) {
	// register messagers
	g.P("func init() {")
	for _, messager := range messagers {
		g.P(`register(&`, messager, `{})`)
	}
	g.P("}")
}

// genMessage generates a message definition.
func genMessage(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, message *protogen.Message) {
	messagerName := string(message.Desc.Name())

	// messager definition
	g.P("type ", messagerName, " struct {")
	g.P(loaderImportPath.Ident(messagerName))
	g.P("}")
	g.P()

	// messager methods
	g.P("func (x *", messagerName, ") Messager() ", loaderImportPath.Ident("Messager"), " {")
	g.P("return &x.", messagerName)
	g.P("}")
	g.P()

	g.P("func (x *", messagerName, ") Check() error {")
	g.P("// TODO: implement this check function.")
	g.P("return nil")
	g.P("}")
	g.P()
}
