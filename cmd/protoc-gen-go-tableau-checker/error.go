package main

import (
	"fmt"
	"path/filepath"

	"google.golang.org/protobuf/compiler/protogen"
)

// generateError generates the error file containing Issue, CheckResult, ErrorFormat, and CheckError types.
func generateError(gen *protogen.Plugin) error {
	errorTemplateBytes, err := efs.ReadFile("embed/templates/error.go.tpl")
	if err != nil {
		return fmt.Errorf("read embedded error.go.tpl: %w", err)
	}
	filename := filepath.Join("error." + checkExt + ".go")
	g := gen.NewGeneratedFile(filename, "")
	generateCommonHeader(gen, g, false)
	g.P()
	g.P("package ", params.pkg)
	g.P("import (")
	g.P(string(errorTemplateBytes))
	g.P()
	return nil
}
