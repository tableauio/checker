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

const staticHubContent = `
	"errors"
	"fmt"
	"sync"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

var registrarSingleton *tableau.Registrar
var once sync.Once

func getRegistrar() *tableau.Registrar {
	once.Do(func() {
		registrarSingleton = tableau.NewRegistrar()
	})
	return registrarSingleton
}

type Hub struct {
	*tableau.Hub
	filteredCheckerMap tableau.MessagerMap
}

func NewHub() *Hub {
	return &Hub{
		Hub:                tableau.NewHub(),
		filteredCheckerMap: tableau.MessagerMap{},
	}
}

func (h *Hub) load(dir string, filter tableau.Filter, format format.Format, options ...Option) error {
	opts := ParseOptions(options...)
	filteredCheckerMap := h.NewMessagerMap(filter)
	for name, gen := range registrarSingleton.Generators {
		if filter == nil || filter.Filter(name) {
			filteredCheckerMap[name] = gen()
		}
	}
	h.filteredCheckerMap = filteredCheckerMap

	var mu sync.Mutex // guard msgers
	msgers := tableau.MessagerMap{}

	var loadOpts []load.Option
	loadOpts = append(loadOpts, load.SubdirRewrites(opts.SubdirRewrites))
	if opts.IgnoreUnknownFields {
		loadOpts = append(loadOpts, load.IgnoreUnknownFields())
	}

	var errsMu sync.Mutex
	var errs []error
	var eg errgroup.Group
	for name, msger := range h.filteredCheckerMap {
		name := name
		msger := msger
		eg.Go(func() error {
			log.Infof("=== LOAD  %s", name)
			if err := msger.Messager().Load(dir, format, loadOpts...); err != nil {
				bookName, sheetName := getBookAndSheet(opts.ProtoPackage, name)
				//lint:ignore ST1005 we want to prettify multiple error messages
				err := fmt.Errorf("error: workbook %s, worksheet %s, load failed: %+v\n", bookName, sheetName, err)
				if opts.SkipLoadErrors {
					errsMu.Lock()
					errs = append(errs, err)
					errsMu.Unlock()
					return nil
				}
				return err
			}
			log.Infof("--- DONE: %v", name)

			mu.Lock()
			msgers[name] = msger.Messager()
			mu.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	h.SetMessagerMap(msgers)
	return errors.Join(errs...)
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

func (h *Hub) check(protoPackage string, breakFailedCount int) error {
	var errs []error
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
				//lint:ignore ST1005 we want to prettify multiple error messages
				err := fmt.Errorf("error: workbook %s, worksheet %s, builtin check failed: %+v\n", bookName, sheetName, err1)
				errs = append(errs, err)
			}
			if err2 != nil {
				//lint:ignore ST1005 we want to prettify multiple error messages
				err := fmt.Errorf("error: workbook %s, worksheet %s, custom check failed: %+v\n", bookName, sheetName, err2)
				errs = append(errs, err)
			}
		} else {
			log.Infof("--- PASS: %v", name)
		}
		if len(errs) >= breakFailedCount {
			break
		}
	}
	return errors.Join(errs...)
}

func (h *Hub) checkCompatibility(newHub *tableau.Hub, protoPackage string, breakFailedCount int) error {
	var errs []error
	for name, checker := range h.filteredCheckerMap {
		log.Infof("=== RUN   %v", name)
		// custom check logic
		err := checker.CheckCompatibility(h.Hub, newHub)
		if err != nil {
			bookName, sheetName := getBookAndSheet(protoPackage, name)
			log.Errorf("--- FAIL: workbook %s, worksheet %s", bookName, sheetName)
			//lint:ignore ST1005 we want to prettify multiple error messages
			err := fmt.Errorf("error: workbook %s, worksheet %s, custom check failed: %+v\n", bookName, sheetName, err)
			errs = append(errs, err)

		} else {
			log.Infof("--- PASS: %v", name)
		}
		if len(errs) >= breakFailedCount {
			break
		}
	}
	return errors.Join(errs...)
}

func (h *Hub) Check(dir string, filter tableau.Filter, format format.Format, options ...Option) error {
	opts := ParseOptions(options...)
	// load hub
	loadErr := h.load(dir, filter, format, options...)
	if loadErr != nil && !opts.SkipLoadErrors {
		return loadErr
	}
	checkErr := h.check(opts.ProtoPackage, opts.BreakFailedCount)
	return errors.Join(loadErr, checkErr)
}

func (h *Hub) CheckCompatibility(dir, newDir string, filter tableau.Filter, format format.Format, options ...Option) error {
	opts := ParseOptions(options...)
	// load hub
	loadErr := h.load(dir, filter, format, options...);
	if loadErr != nil && !opts.SkipLoadErrors {
		return loadErr
	}
	// load new hub
	newHub := NewHub()
	loadErr1 := newHub.load(newDir, filter, format, options...)
	if loadErr1 != nil && !opts.SkipLoadErrors {
		return loadErr1
	}
	checkErr := h.checkCompatibility(newHub.Hub, opts.ProtoPackage, opts.BreakFailedCount)
	return errors.Join(loadErr, loadErr1, checkErr)
}

func register(gen tableau.MessagerGenerator) {
	getRegistrar().Register(gen)
}

type Options struct {
	// Break check loop if failed count is equal to or more than BreakFailedCount.
	//
	// Default: 1.
	BreakFailedCount int
	// Rewrite subdir path (relative to workbook name option in .proto file).
	//
	// Default: nil.
	SubdirRewrites map[string]string
	// The proto package name of .proto files.
	//
	// Default: "protoconf".
	ProtoPackage string
	// Whether to ignore unknown JSON fields during parsing.
	//
	// Default: false.
	IgnoreUnknownFields bool
	// Whether to ignore errors during loading.
	//
	// Errors may occur during loading old config files when do compatibility
	// check. For example, some new worksheets you recently add are not
	// existed, or proto schema are not compatible, just ignore the loading
	// errors (then these proto message objects are nil after loading), so that
	// compatibility check can continue to run.
	//
	// Default: false.
	SkipLoadErrors bool
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

// ProtoPackage sets ProtoPackage option.
func ProtoPackage(protoPackage string) Option {
	return func(opts *Options) {
		opts.ProtoPackage = protoPackage
	}
}

// IgnoreUnknownFields sets IgnoreUnknownFields option as true.
func IgnoreUnknownFields() Option {
	return func(opts *Options) {
		opts.IgnoreUnknownFields = true
	}
}

// SkipLoadErrors sets SkipLoadErrors option as true.
func SkipLoadErrors() Option {
	return func(opts *Options) {
		opts.SkipLoadErrors = true
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
