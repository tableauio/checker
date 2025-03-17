package main

import (
	"os"
	"strings"

	"github.com/tableauio/checker/test/check"
	"github.com/tableauio/checker/test/protoconf/tableau"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
)

var protoPkg = "protoconf"
var pathPrefix = ""

func Filter(messagerName string) bool {
	fullName := protoreflect.FullName(protoPkg + "." + messagerName)
	mt, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
	if err != nil {
		log.Panicf("failed to find messager %s: %+v", fullName, err)
	}
	fd := mt.Descriptor().ParentFile()
	opts := fd.Options().(*descriptorpb.FileOptions)
	workbook := proto.GetExtension(opts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	return strings.HasPrefix(workbook.Name, pathPrefix)
}

func main() {
	log.Init(&log.Options{
		Mode:     "FULL",
		Level:    "INFO",
		Filename: "_logs/checker.log",
		Sink:     "MULTI",
	})
	err1 := check.NewHub(tableau.Filter(Filter)).Check("./testdata/", format.JSON,
		check.BreakFailedCount(2),
		check.WithLoadOptions(load.IgnoreUnknownFields()),
	)
	if err1 != nil {
		log.Errorf("check failed, see errors below:\n%v", err1)
		os.Exit(-1)
	}
	err2 := check.NewHub(tableau.Filter(Filter)).CheckCompatibility("./testdata/", "./testdata1/", format.JSON,
		check.SkipLoadErrors(),
		check.BreakFailedCount(2),
		check.WithLoadOptions(load.IgnoreUnknownFields()),
	)
	if err2 != nil {
		log.Errorf("check compatibility failed, see errors below:\n%v", err2)
		os.Exit(-1)
	}
}
