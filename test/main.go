package main

import (
	"os"
	"strings"

	"github.com/tableauio/checker/test/check"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	// check "github.com/tableauio/checker/test/devcheck"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
)

var protoPkg = "protoconf"
var pathPrefix = ""

type checkFilter struct {
}

func (cf *checkFilter) Filter(messagerName string) bool {
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
	err1 := check.NewHub().Check("./testdata/", &checkFilter{}, format.JSON,
		check.BreakFailedCount(2),
		check.IgnoreUnknownFields())
	if err1 != nil {
		log.Errorf("check failed, see errors below:\n%s", xerrors.NewDesc(err1))
		os.Exit(-1)
	}
	err2 := check.NewHub().CheckCompatibility("./testdata/", "./testdata1/", &checkFilter{}, format.JSON,
		check.BreakFailedCount(2),
		check.IgnoreUnknownFields())
	if err2 != nil {
		log.Errorf("check compatibility failed, see errors below:\n%s", xerrors.NewDesc(err2))
		os.Exit(-1)
	}
}
