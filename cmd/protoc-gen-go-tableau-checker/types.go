package main

import (
	"path/filepath"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// generateTypes generates related types files.
func generateTypes(gen *protogen.Plugin) {
	filename := filepath.Join("types." + checkExt + ".go")
	g := gen.NewGeneratedFile(filename, "")
	generateCommonHeader(gen, g)
	g.P()
	g.P("package ", params.pkg)
	var messages []*protogen.Message
	for _, f := range gen.Files {
		if !NeedGenFile(f) {
			continue
		}
		for _, message := range f.Messages {
			opts := message.Desc.Options().(*descriptorpb.MessageOptions)
			worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
			if worksheet != nil {
				messages = append(messages, message)
			}
		}
	}
	for _, message := range messages {
		messagerName := string(message.Desc.Name())
		g.P()
		// messager definition
		g.P("type ", messagerName, " struct {")
		g.P(loaderImportPath.Ident(messagerName))
		g.P("}")
	}
	g.P()
	// register messagers
	g.P("func init() {")
	for _, message := range messages {
		messagerName := string(message.Desc.Name())
		g.P("register(func() checker {")
		g.P("return new(", messagerName, ")")
		g.P("})")
	}
	g.P("}")
}
