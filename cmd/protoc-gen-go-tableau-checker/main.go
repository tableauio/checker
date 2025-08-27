package main

import (
	"flag"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

const version = "0.6.1"

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
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		for _, f := range gen.Files {
			if !NeedGenFile(f) {
				continue
			}
			generateMessager(gen, f)
		}
		generateHub(gen)
		return nil
	})
}
