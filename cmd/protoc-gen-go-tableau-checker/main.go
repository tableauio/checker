package main

import (
	"flag"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

const version = "0.1.6"

type Params struct {
	pkg       string
	loaderPkg string
	outdir    string
}

var params = Params{}

func main() {
	var flags flag.FlagSet
	flags.StringVar(&params.pkg, "pkg", "check", "tableau checker package name")
	flags.StringVar(&params.loaderPkg, "loader-pkg", "tableau", "tableau loader package name")
	flags.StringVar(&params.outdir, "out", "", "tableau checker output directory")
	flag.Parse()

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}

			opts := f.Desc.Options().(*descriptorpb.FileOptions)
			workbook := proto.GetExtension(opts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
			if workbook == nil {
				continue
			}
			generateMessager(gen, f)
		}
		generateHub(gen)
		return nil
	})
}
