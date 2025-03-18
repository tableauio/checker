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
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Checker interface {
	tableau.Messager
	Messager() tableau.Messager
	Check(hub *tableau.Hub) error
	CheckCompatibility(hub, newHub *tableau.Hub) error
}

type CheckerGenerator = func() Checker
type registrar struct {
	Generators map[string]CheckerGenerator
}

func (r *registrar) Register(gen CheckerGenerator) {
	if _, ok := r.Generators[gen().Name()]; ok {
		panic("register duplicate checker: " + gen().Name())
	}
	r.Generators[gen().Name()] = gen
}

var registrarSingleton *registrar
var once sync.Once

func getRegistrar() *registrar {
	once.Do(func() {
		registrarSingleton = &registrar{
			Generators: map[string]CheckerGenerator{},
		}
	})
	return registrarSingleton
}

func register(gen CheckerGenerator) {
	getRegistrar().Register(gen)
}

type Hub struct {
	*tableau.Hub
	checkers map[string]Checker
}

func NewHub(options ...tableau.Option) *Hub {
	return &Hub{
		Hub:      tableau.NewHub(options...),
		checkers: map[string]Checker{},
	}
}

const (
	loadTypeDefault = ""
	loadTypeOld     = "(old)"
	loadTypeNew     = "(new)"
)

func (h *Hub) load(loadType, protoPackage, dir string, f format.Format, options ...load.Option) error {
	var mu sync.Mutex
	msgers := tableau.MessagerMap{}
	var errs []error
	var wg sync.WaitGroup
	for name, msger := range h.NewMessagerMap() {
		name := name
		msger := msger
		if gen, ok := registrarSingleton.Generators[name]; ok {
			checker := gen()
			h.checkers[name] = checker
			msger = checker.Messager()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Infof("=== LOAD  %v%v", name, loadType)
			if err := msger.Load(dir, f, options...); err != nil {
				bookName, sheetName := getBookAndSheet(protoPackage, name)
				//lint:ignore ST1005 we want to prettify multiple error messages
				err := fmt.Errorf("error: workbook %s, worksheet %s, load failed: %+v\n", bookName, sheetName, xerrors.NewDesc(err).ErrString(false))
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				log.Infof("--- FAIL: %v%v", name, loadType)
			} else {
				mu.Lock()
				msgers[name] = msger
				mu.Unlock()
				log.Infof("--- DONE: %v%v", name, loadType)
			}
		}()
	}
	wg.Wait()
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

	worksheet, ok := proto.GetExtension(mt.Descriptor().Options(), tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	if !ok {
		log.Errorf("messager %s does not belong to any worksheet", fullName)
		return "", ""
	}

	fd := mt.Descriptor().ParentFile()
	workbook, ok := proto.GetExtension(fd.Options(), tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	if !ok {
		log.Errorf("messager %s does not belong to any workbook", fullName)
		return "", ""
	}

	return workbook.GetName(), worksheet.GetName()
}

func (h *Hub) check(protoPackage string, breakFailedCount int) error {
	var errs []error
	for name, checker := range h.checkers {
		log.Infof("=== RUN   %v", name)
		// custom check logic
		err := checker.Check(h.Hub)
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

func (h *Hub) checkCompatibility(newHub *tableau.Hub, protoPackage string, breakFailedCount int) error {
	var errs []error
	for name, checker := range h.checkers {
		if h.GetMessager(name) == nil || newHub.GetMessager(name) == nil {
			log.Infof("=== SKIP  %v", name)
			continue
		}
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

func (h *Hub) Check(dir string, format format.Format, options ...Option) error {
	opts := ParseOptions(options...)
	// load hub
	err := h.load(loadTypeDefault, opts.ProtoPackage, dir, format, opts.LoadOptions...)
	if err != nil {
		return err
	}
	return h.check(opts.ProtoPackage, opts.BreakFailedCount)
}

func (h *Hub) CheckCompatibility(dir, newDir string, format format.Format, options ...Option) error {
	opts := ParseOptions(options...)
	// load new hub
	loadErr := h.load(loadTypeNew, opts.ProtoPackage, newDir, format, opts.LoadOptions...)
	if loadErr != nil && !opts.SkipLoadErrors {
		return loadErr
	}
	newHub := tableau.NewHub()
	newHub.SetMessagerMap(h.GetMessagerMap())
	// load hub
	loadErr1 := h.load(loadTypeOld, opts.ProtoPackage, dir, format, opts.LoadOptions...)
	if loadErr1 != nil && !opts.SkipLoadErrors {
		return loadErr1
	}
	checkErr := h.checkCompatibility(newHub, opts.ProtoPackage, opts.BreakFailedCount)
	return errors.Join(loadErr, loadErr1, checkErr)
}

type Options struct {
	// Break check loop if failed count is equal to or more than BreakFailedCount.
	//
	// Default: 1.
	BreakFailedCount int
	// The proto package name of .proto files.
	//
	// Default: "protoconf".
	ProtoPackage string
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
	// Options for messager loading.
	//
	// Default: nil.
	LoadOptions []load.Option
}

// Option is the functional option type.
type Option func(*Options)

// BreakFailedCount sets BreakFailedCount option.
func BreakFailedCount(count int) Option {
	return func(opts *Options) {
		opts.BreakFailedCount = count
	}
}

// ProtoPackage sets ProtoPackage option.
func ProtoPackage(protoPackage string) Option {
	return func(opts *Options) {
		opts.ProtoPackage = protoPackage
	}
}

// SkipLoadErrors sets SkipLoadErrors option as true.
func SkipLoadErrors() Option {
	return func(opts *Options) {
		opts.SkipLoadErrors = true
	}
}

// WithLoadOptions sets options for messager loading.
func WithLoadOptions(options ...load.Option) Option {
	return func(opts *Options) {
		opts.LoadOptions = options
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
