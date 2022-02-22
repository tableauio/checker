package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
var loaderImportPath protogen.GoImportPath

// generateMessager generates a protoconf file correponsing to the protobuf file.
// Each wrapped struct type implement the Messager interface.
func generateMessager(gen *protogen.Plugin, file *protogen.File) {
	loaderImportPath = protogen.GoImportPath(string(file.GoImportPath) + "/" + *loaderPkg)

	filename := filepath.Join(file.GeneratedFilenamePrefix + "." + checkExt + ".go")
	path := filepath.Join(*out, filename)
	existed, err := Exists(path)
	if err != nil {
		panic(err)
	}
	if existed {
		g := gen.NewGeneratedFile(filename, "")
		generateFileHeader(gen, file, g, false)
		addIncrementalFileContent(gen, file, g, path)
	} else {
		g := gen.NewGeneratedFile(filename, "")
		generateFileHeader(gen, file, g, false)
		g.P()
		g.P("package ", *pkg)
		g.P()
		generateFileContent(gen, file, g)
	}
}

var checkerRegexp *regexp.Regexp

func init() {
	checkerRegexp = regexp.MustCompile(`^type (.+) struct {`) // e.g.: map<uint32,Type>
}

func addIncrementalFileContent(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, path string) {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	messagerMap := map[string]bool{}
	var fileMessagers []string
	for _, message := range file.Messages {
		opts := message.Desc.Options().(*descriptorpb.MessageOptions)
		worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
		if worksheet != nil {
			messagerName := string(message.Desc.Name())
			messagerMap[messagerName] = false
			fileMessagers = append(fileMessagers, messagerName)
		}
	}
	content := ""
	scanner := bufio.NewScanner(f)
	line := 0
	headingCommentLines := 0
	initFirstLine := -1
	initEndLine := -1
	for scanner.Scan() {
		line++
		if headingCommentLines+1 == line && strings.HasPrefix(scanner.Text(), "//") {
			headingCommentLines++
		}
		if line > headingCommentLines {
			if strings.HasPrefix(scanner.Text(), "func init()") {
				initFirstLine = line
			}
			if initFirstLine > 0 && initEndLine < 0 {
				if strings.HasPrefix(scanner.Text(), "}") {
					initEndLine = line
				}
			}
			if initFirstLine < 0 || (initEndLine > 0 && line > initEndLine) {
				content += scanner.Text() + "\n"
			}
		}
		if matches := checkerRegexp.FindStringSubmatch(scanner.Text()); len(matches) > 0 {
			msger := strings.TrimSpace(matches[1])
			if _, ok := messagerMap[msger]; ok {
				messagerMap[msger] = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	g.P(content)
	for messagerName, existed := range messagerMap {
		if !existed {
			genMessage(gen, file, g, messagerName, true)
		}
	}

	generateRegister(fileMessagers, g)
}

// generateFileContent generates struct type definitions.
func generateFileContent(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile) {
	var fileMessagers []string
	for _, message := range file.Messages {
		opts := message.Desc.Options().(*descriptorpb.MessageOptions)
		worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
		if worksheet != nil {
			messagerName := string(message.Desc.Name())
			genMessage(gen, file, g, messagerName, false)
			fileMessagers = append(fileMessagers, messagerName)
		}
	}
	generateRegister(fileMessagers, g)
}

func generateRegister(messagers []string, g *protogen.GeneratedFile) {
	// register messagers
	g.P("func init() {")
	g.P("// NOTE: This func is auto-generated. DO NOT EDIT.")
	for _, messager := range messagers {
		g.P(`register(&`, messager, `{})`)
	}
	g.P("}")
}

// genMessage generates a message definition.
func genMessage(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, messagerName string, incremental bool) {
	// messager definition
	g.P("type ", messagerName, " struct {")
	if incremental {
		g.P(*loaderPkg, ".", messagerName)
	} else {
		g.P(loaderImportPath.Ident(messagerName))
	}
	g.P("}")
	g.P()

	g.P("func (x *", messagerName, ") Check() error {")
	g.P("// TODO: implement this check function.")
	g.P("return nil")
	g.P("}")
	g.P()
}
