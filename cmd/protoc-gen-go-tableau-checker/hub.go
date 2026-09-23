package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
)

// generateHub generates the hub file containing Hub load/check logic.
func generateHub(gen *protogen.Plugin) error {
	if !loaderImportPathSet {
		return fmt.Errorf("no tableau workbook files generated; cannot determine loader import path for hub.check.go")
	}
	hubTemplateBytes, err := efs.ReadFile("embed/templates/hub.go.tpl")
	if err != nil {
		return fmt.Errorf("read embedded hub.go.tpl: %w", err)
	}
	tpl, err := template.New("hub").Parse(string(hubTemplateBytes))
	if err != nil {
		return fmt.Errorf("parse hub.go.tpl: %w", err)
	}
	var body bytes.Buffer
	if err := tpl.Execute(&body, map[string]string{
		"LoaderImport": loaderImportPath.String(),
	}); err != nil {
		return fmt.Errorf("execute hub.go.tpl: %w", err)
	}

	filename := filepath.Join("hub." + checkExt + ".go")
	g := gen.NewGeneratedFile(filename, "")
	generateCommonHeader(gen, g, true)
	g.P()
	g.P("package ", params.pkg)
	g.P()
	g.P(body.String())
	return nil
}
