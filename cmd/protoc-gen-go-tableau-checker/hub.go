package main

import (
	"fmt"
	"path/filepath"

	"google.golang.org/protobuf/compiler/protogen"
)

// generateHub generates the hub file containing Issue, CheckResult types and Hub logic.
func generateHub(gen *protogen.Plugin) error {
	hubTemplateBytes, err := efs.ReadFile("embed/templates/hub.go.tpl")
	if err != nil {
		return fmt.Errorf("read embedded hub.go.tpl: %w", err)
	}
	filename := filepath.Join("hub." + checkExt + ".go")
	g := gen.NewGeneratedFile(filename, "")
	generateCommonHeader(gen, g, true)
	g.P()
	g.P("package ", params.pkg)
	g.P("import (")
	g.P("tableau ", loaderImportPath)
	g.P()
	g.P(string(hubTemplateBytes))
	g.P()
	return nil
}
