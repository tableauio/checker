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
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// IssueKind represents the kind of check issue.
type IssueKind string

const (
	// IssueKindLoad represents an issue that occurred during loading.
	IssueKindLoad IssueKind = "load"
	// IssueKindCheck represents an issue that occurred during custom check.
	IssueKindCheck IssueKind = "check"
	// IssueKindCompatibility represents an issue that occurred during compatibility check.
	IssueKindCompatibility IssueKind = "compatibility"
)

// Issue represents a single structured check error.
type Issue struct {
	Kind      IssueKind                   ` + "`" + `json:"kind"` + "`" + `
	Message   string                      ` + "`" + `json:"message"` + "`" + `
	Workbook  *tableaupb.WorkbookOptions  ` + "`" + `json:"workbook,omitempty"` + "`" + `
	Worksheet *tableaupb.WorksheetOptions ` + "`" + `json:"worksheet,omitempty"` + "`" + `
}

// Error implements the error interface.
// The format is identical to the original error messages produced before
// structured issues were introduced, ensuring backward-compatible output
// when errors are printed via errors.Join or %v/%s formatting.
func (i Issue) Error() string {
	return fmt.Sprintf("error: workbook %s (%s), worksheet %s (%s), %s",
		i.Workbook.GetName(), i.Workbook,
		i.Worksheet.GetName(), i.Worksheet,
		i.Message)
}

// protoJSONMarshaler is used to serialize proto messages with correct field names.
// encoding/json cannot correctly serialize proto messages on its own (e.g. enum
// values would be emitted as integers rather than their string names).
var protoJSONMarshaler = protojson.MarshalOptions{EmitUnpopulated: false}

// MarshalJSON implements json.Marshaler.
// Proto fields are serialized via protojson to produce correct field names,
// because encoding/json cannot correctly serialize proto messages on its own.
func (i Issue) MarshalJSON() ([]byte, error) {
	out := struct {
		Kind      IssueKind       ` + "`" + `json:"kind"` + "`" + `
		Message   string          ` + "`" + `json:"message"` + "`" + `
		Workbook  json.RawMessage ` + "`" + `json:"workbook,omitempty"` + "`" + `
		Worksheet json.RawMessage ` + "`" + `json:"worksheet,omitempty"` + "`" + `
	}{
		Kind:    i.Kind,
		Message: i.Message,
	}
	if i.Workbook != nil {
		b, err := protoJSONMarshaler.Marshal(i.Workbook)
		if err != nil {
			return nil, err
		}
		out.Workbook = json.RawMessage(b)
	}
	if i.Worksheet != nil {
		b, err := protoJSONMarshaler.Marshal(i.Worksheet)
		if err != nil {
			return nil, err
		}
		out.Worksheet = json.RawMessage(b)
	}
	return json.Marshal(out)
}

// ErrorFormat is a function that formats a slice of issues into a string.
// It is called by CheckError.Error() to produce the final error message.
type ErrorFormat func([]Issue) string

// ErrorFormatText formats issues as human-readable text lines (default).
var ErrorFormatText ErrorFormat = func(issues []Issue) string {
	msgs := make([]string, len(issues))
	for i, issue := range issues {
		msgs[i] = issue.Error()
	}
	return strings.Join(msgs, "\n")
}

// ErrorFormatJSON formats issues as a JSON object with an "issues" array.
// Falls back to ErrorFormatText if marshaling fails.
var ErrorFormatJSON ErrorFormat = func(issues []Issue) string {
	b, err := json.Marshal(struct {
		Issues []Issue ` + "`" + `json:"issues"` + "`" + `
	}{Issues: issues})
	if err != nil {
		log.Errorf("failed to marshal issues to JSON, falling back to text format: %+v", err)
		return ErrorFormatText(issues)
	}
	return string(b)
}

// CheckError is the error type returned by Check and CheckCompatibility.
// Its Error() method serializes all collected issues in the configured format,
// so fmt.Println(err) / log.Error(err) produce the right output without any
// extra glue code on the caller side.
type CheckError struct {
	Issues []Issue
	Format ErrorFormat
}

// Error implements the error interface, formatting issues via the configured ErrorFormat.
// Falls back to ErrorFormatText if Format is nil.
func (e *CheckError) Error() string {
	if e.Format == nil {
		return ErrorFormatText(e.Issues)
	}
	return e.Format(e.Issues)
}

// Unwrap returns the individual issues as errors, enabling errors.Is/As traversal.
func (e *CheckError) Unwrap() []error {
	errs := make([]error, len(e.Issues))
	for i, issue := range e.Issues {
		errs[i] = issue
	}
	return errs
}

type checker interface {
	tableau.Messager
	Check(hub *tableau.Hub) error
	CheckCompatibility(hub, newHub *tableau.Hub) error
}

type checkerGenerator = func() checker
type registrar struct {
	Generators map[string]checkerGenerator
}

func (r *registrar) Register(gen checkerGenerator) {
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
			Generators: map[string]checkerGenerator{},
		}
	})
	return registrarSingleton
}

func register(gen checkerGenerator) {
	getRegistrar().Register(gen)
}

type Hub struct {
	*tableau.Hub
	checkers map[string]checker
}

func NewHub(options ...tableau.Option) *Hub {
	return &Hub{
		Hub:      tableau.NewHub(options...),
		checkers: map[string]checker{},
	}
}

const (
	loadTypeDefault = ""
	loadTypeOld     = "(old)"
	loadTypeNew     = "(new)"
)

func (h *Hub) load(loadType, protoPackage, dir string, f format.Format, options ...load.Option) []Issue {
	var mu sync.Mutex
	msgers := tableau.MessagerMap{}
	var issues []Issue
	var wg sync.WaitGroup
	opts := load.ParseOptions(options...)
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
			mopts := opts.ParseMessagerOptionsByName(name)
			if err := msger.Load(dir, f, mopts); err != nil {
				workbook, worksheet := getBookAndSheet(protoPackage, name)
				issue := Issue{
					Kind:      IssueKindLoad,
					Message:   fmt.Sprintf("load failed: %+v", xerrors.NewDesc(err).ErrString(false)),
					Workbook:  workbook,
					Worksheet: worksheet,
				}
				mu.Lock()
				issues = append(issues, issue)
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
	return issues
}

func getBookAndSheet(protoPackage, msgName string) (*tableaupb.WorkbookOptions, *tableaupb.WorksheetOptions) {
	fullName := protoreflect.FullName(protoPackage + "." + msgName)
	mt, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
	if err != nil {
		log.Errorf("failed to find messager %s: %+v", fullName, err)
		return nil, nil
	}

	worksheet, ok := proto.GetExtension(mt.Descriptor().Options(), tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	if !ok {
		log.Errorf("messager %s does not belong to any worksheet", fullName)
		return nil, nil
	}

	fd := mt.Descriptor().ParentFile()
	workbook, ok := proto.GetExtension(fd.Options(), tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	if !ok {
		log.Errorf("messager %s does not belong to any workbook", fullName)
		return nil, nil
	}

	return workbook, worksheet
}

func (h *Hub) check(protoPackage string, breakFailedCount int) []Issue {
	var issues []Issue
	for name, checker := range h.checkers {
		log.Infof("=== RUN   %v", name)
		// custom check logic
		err := checker.Check(h.Hub)
		if err != nil {
			workbook, worksheet := getBookAndSheet(protoPackage, name)
			log.Errorf("--- FAIL: workbook %s, worksheet %s", workbook.GetName(), worksheet.GetName())
			issues = append(issues, Issue{
				Kind:      IssueKindCheck,
				Message:   fmt.Sprintf("custom check failed: %+v", err),
				Workbook:  workbook,
				Worksheet: worksheet,
			})
		} else {
			log.Infof("--- PASS: %v", name)
		}
		if len(issues) >= breakFailedCount {
			break
		}
	}
	return issues
}

func (h *Hub) checkCompatibility(newHub *tableau.Hub, protoPackage string, breakFailedCount int) []Issue {
	var issues []Issue
	for name, checker := range h.checkers {
		if h.GetMessager(name) == nil || newHub.GetMessager(name) == nil {
			log.Infof("=== SKIP  %v", name)
			continue
		}
		log.Infof("=== RUN   %v", name)
		// custom check logic
		err := checker.CheckCompatibility(h.Hub, newHub)
		if err != nil {
			workbook, worksheet := getBookAndSheet(protoPackage, name)
			log.Errorf("--- FAIL: workbook %s, worksheet %s", workbook.GetName(), worksheet.GetName())
			issues = append(issues, Issue{
				Kind:      IssueKindCompatibility,
				Message:   fmt.Sprintf("custom check failed: %+v", err),
				Workbook:  workbook,
				Worksheet: worksheet,
			})
		} else {
			log.Infof("--- PASS: %v", name)
		}
		if len(issues) >= breakFailedCount {
			break
		}
	}
	return issues
}

func (h *Hub) Check(dir string, format format.Format, options ...Option) error {
	opts := ParseOptions(options...)
	// load hub
	loadIssues := h.load(loadTypeDefault, opts.ProtoPackage, dir, format, opts.LoadOptions...)
	if len(loadIssues) > 0 {
		return &CheckError{Issues: loadIssues, Format: opts.ErrorFormat}
	}
	checkIssues := h.check(opts.ProtoPackage, opts.BreakFailedCount)
	if len(checkIssues) > 0 {
		return &CheckError{Issues: checkIssues, Format: opts.ErrorFormat}
	}
	return nil
}

func (h *Hub) CheckCompatibility(dir, newDir string, format format.Format, options ...Option) error {
	opts := ParseOptions(options...)
	// load new hub
	newLoadIssues := h.load(loadTypeNew, opts.ProtoPackage, newDir, format, opts.LoadOptions...)
	if len(newLoadIssues) > 0 && !opts.SkipLoadErrors {
		return &CheckError{Issues: newLoadIssues, Format: opts.ErrorFormat}
	}
	newHub := tableau.NewHub()
	newHub.SetMessagerMap(h.GetMessagerMap())
	// load hub
	oldLoadIssues := h.load(loadTypeOld, opts.ProtoPackage, dir, format, opts.LoadOptions...)
	if len(oldLoadIssues) > 0 && !opts.SkipLoadErrors {
		return &CheckError{Issues: append(newLoadIssues, oldLoadIssues...), Format: opts.ErrorFormat}
	}
	compatIssues := h.checkCompatibility(newHub, opts.ProtoPackage, opts.BreakFailedCount)
	allIssues := append(append(newLoadIssues, oldLoadIssues...), compatIssues...)
	if len(allIssues) > 0 {
		return &CheckError{Issues: allIssues, Format: opts.ErrorFormat}
	}
	return nil
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
	// ErrorFormat controls how the returned error is formatted when printed.
	// Use ErrorFormatText (default) or ErrorFormatJSON for built-in formats,
	// or provide a custom func([]Issue) string for fully custom formatting.
	//
	// Default: ErrorFormatText.
	ErrorFormat ErrorFormat
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

// WithErrorFormat sets the format used when the returned error is printed.
// Use ErrorFormatText (default) or ErrorFormatJSON for built-in formats,
// or pass a custom func([]Issue) string for fully custom formatting.
func WithErrorFormat(f ErrorFormat) Option {
	return func(opts *Options) {
		opts.ErrorFormat = f
	}
}

// newDefault returns a default Options.
func newDefault() *Options {
	return &Options{
		BreakFailedCount: 1,
		ProtoPackage:     "protoconf",
		ErrorFormat:      ErrorFormatText,
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
