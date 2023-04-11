package main

import (
	"path/filepath"

	"google.golang.org/protobuf/compiler/protogen"
)

// generateHub generates related hub files.
func generateHub(gen *protogen.Plugin) {
	filename := filepath.Join("hub." + checkExt + ".go")
	g := gen.NewGeneratedFile(filename, "")
	generateCommonHeader(gen, g, true)
	g.P()
	g.P("package ", params.pkg)
	g.P("import (")
	g.P("tableau ", loaderImportPath)
	g.P()
	g.P(staticHubContent)
	g.P()
}

const staticHubContent = `"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
)

type Hub struct {
	*tableau.Hub
	checkerMap         tableau.MessagerMap
	filteredCheckerMap tableau.MessagerMap
}

var hubSingleton *Hub
var once sync.Once

// GetHub return the singleton of Hub
func GetHub() *Hub {
	once.Do(func() {
		// new instance
		hubSingleton = &Hub{
			Hub:                tableau.NewHub(),
			checkerMap:         tableau.MessagerMap{},
			filteredCheckerMap: tableau.MessagerMap{},
		}
	})
	return hubSingleton
}

func (h *Hub) Register(msger tableau.Messager) error {
	h.checkerMap[msger.Messager().Name()] = msger
	return nil
}

func (h *Hub) load(dir string, format format.Format, opts *Options) error {
	var mu sync.Mutex // guard msgers
	msgers := tableau.MessagerMap{}

	var eg errgroup.Group
	for name, msger := range h.filteredCheckerMap {
		name := name
		msger := msger
		eg.Go(func() error {
			log.Infof("=== LOAD  %s", name)
			if err := msger.Messager().Load(dir, format,
				load.SubdirRewrites(opts.SubdirRewrites),
				load.IgnoreUnknownFields(opts.IgnoreUnknownFields)); err != nil {
				bookName, sheetName := getBookAndSheet(opts.ProtoPackage, name)
				log.Errorf("--- FAIL: workbook %s, worksheet %s", bookName, sheetName)
				log.Errorf("load error:%+v, workbook %s, worksheet %s", err, bookName, sheetName)
				return errors.WithMessagef(err, "failed to load %v", name)
			}
			log.Infof("--- DONE: %v", name)

			mu.Lock()
			msgers[name] = msger.Messager()
			mu.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		log.Errorf("--- FAIL: load failed: %v", err)
		return err
	}
	h.SetMessagerMap(msgers)
	return nil
}

func getBookAndSheet(protoPackage, msgName string) (bookName string, sheetName string) {
	fullName := protoreflect.FullName(protoPackage + "." + msgName)
	mt, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
	if err != nil {
		log.Errorf("failed to find messager %s: %+v", fullName, err)
		return "", ""
	}
	mopts := mt.Descriptor().Options()
	worksheet := proto.GetExtension(mopts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)

	fd := mt.Descriptor().ParentFile()
	opts := fd.Options().(*descriptorpb.FileOptions)
	workbook := proto.GetExtension(opts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)

	return workbook.GetName(), worksheet.GetName()
}

func (h *Hub) check(protoPackage string, breakFailedCount int) int {
	failedCount := 0
	for name, checker := range h.filteredCheckerMap {
		log.Infof("=== RUN   %v", name)
		// built-in auto-generated check logic
		err1 := checker.Messager().Check(h.Hub)
		// custom check logic
		err2 := checker.Check(h.Hub)
		if err1 != nil || err2 != nil {
			bookName, sheetName := getBookAndSheet(protoPackage, name)
			log.Errorf("--- FAIL: workbook %s, worksheet %s", bookName, sheetName)
			if err1 != nil {
				log.Errorf("auto check error: %+v, workbook %s, worksheet %s", err1, bookName, sheetName)
			}
			if err2 != nil {
				log.Errorf("custom check error: %+v, workbook %s, worksheet %s", err2, bookName, sheetName)
			}
			failedCount++
		} else {
			log.Infof("--- PASS: %v", name)
		}
		if failedCount != 0 && failedCount >= breakFailedCount {
			break
		}
	}
	return failedCount
}

func (h *Hub) Run(dir string, filter tableau.Filter, format format.Format, options ...Option) error {
	opts := ParseOptions(options...)

	filteredCheckerMap := h.NewMessagerMap(filter)
	for name, msger := range h.checkerMap {
		if filter == nil || filter.Filter(name) {
			filteredCheckerMap[name] = msger
		}
	}
	h.filteredCheckerMap = filteredCheckerMap

	// load
	err := h.load(dir, format, opts)
	if err != nil {
		return err
	}
	// check
	failedCount := h.check(opts.ProtoPackage, opts.BreakFailedCount)
	if failedCount != 0 {
		return fmt.Errorf("Check failed count: %d", failedCount)
	}
	return nil
}

// Syntatic sugar for Hub's register
func register(msger tableau.Messager) {
	GetHub().Register(msger)
}

type Options struct {
	// Break check loop if failed count is equal to or more than BreakFailedCount.
	// Default: 1.
	BreakFailedCount int
	// Rewrite subdir path (relative to workbook name option in .proto file).
	// Default: nil.
	SubdirRewrites map[string]string
	// The proto package name of .proto files.
	// Default: "protoconf".
	ProtoPackage string
	// Whether to ignore unknown JSON fields during parsing.
	// Default: false.
	IgnoreUnknownFields bool
}

// Option is the functional option type.
type Option func(*Options)

// BreakFailedCount sets BreakFailedCount option.
func BreakFailedCount(count int) Option {
	return func(opts *Options) {
		opts.BreakFailedCount = count
	}
}

// SubdirRewrites sets SubdirRewrites option.
func SubdirRewrites(subdirRewrites map[string]string) Option {
	return func(opts *Options) {
		opts.SubdirRewrites = subdirRewrites
	}
}

// newDefault returns a default Options.
func newDefault() *Options {
	return &Options{
		BreakFailedCount: 1,
		ProtoPackage:     "protoconf",
	}
}

// ParseOptions parses functional options and merge them to default Options.
func ParseOptions(setters ...Option) *Options {
	// Default Options
	opts := newDefault()
	for _, setter := range setters {
		setter(opts)
	}
	if opts.BreakFailedCount <= 1 {
		opts.BreakFailedCount = 1
	}
	return opts
}
`
