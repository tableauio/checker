
import (
	tableau {{.LoaderImport}}

	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
)

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
	msger := gen()
	name := msger.Name()
	if _, ok := r.Generators[name]; ok {
		panic("register duplicate checker: " + name)
	}
	r.Generators[name] = gen
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

// load loads all messagers and returns the checker instances that participated
// in this load (keyed by messager name). Callers must assign the returned
// checkers to Hub.checkers when those checkers should drive subsequent Check /
// CheckCompatibility runs. This avoids CheckCompatibility's second load from
// silently overwriting the first load's checkers mid-flight.
func (h *Hub) load(loadType, dir string, f format.Format, options ...load.Option) (map[string]checker, []*Issue) {
	opts := load.ParseOptions(options...)
	messagerMap := h.NewMessagerMap()
	checkers := make(map[string]checker)

	type loadResult struct {
		name  string
		msger tableau.Messager
		issue *Issue
	}
	results := make(chan loadResult, len(messagerMap))
	var wg sync.WaitGroup
	for name, msger := range messagerMap {
		if gen, ok := getRegistrar().Generators[name]; ok {
			c := gen()
			checkers[name] = c
			msger = c.Messager()
		}
		wg.Add(1)
		go func(name string, msger tableau.Messager) {
			defer wg.Done()
			log.Infof("=== LOAD  %v%v", name, loadType)
			mopts := opts.ParseMessagerOptionsByName(name)
			if err := msger.Load(dir, f, mopts); err != nil {
				workbook, worksheet := getBookAndSheet(msger)
				log.Infof("--- FAIL: %v%v", name, loadType)
				results <- loadResult{
					name: name,
					issue: &Issue{
						Kind:      IssueKindLoad,
						Message:   fmt.Sprintf("load failed: %s", err.Error()),
						Workbook:  workbook,
						Worksheet: worksheet,
					},
				}
				return
			}
			log.Infof("--- DONE: %v%v", name, loadType)
			results <- loadResult{name: name, msger: msger}
		}(name, msger)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	msgers := make(tableau.MessagerMap, len(messagerMap))
	issues := make([]*Issue, 0, len(messagerMap))
	for r := range results {
		if r.issue != nil {
			issues = append(issues, r.issue)
			continue
		}
		msgers[r.name] = r.msger
	}
	h.SetMessagerMap(msgers)
	// Align with tableau.Hub.Load: after all messagers are loaded, run
	// ProcessAfterLoadAll so derived messagers (e.g. custom conf indexes) can build.
	if len(issues) == 0 {
		for _, name := range slices.Sorted(maps.Keys(msgers)) {
			msger := msgers[name]
			if err := msger.ProcessAfterLoadAll(h.Hub); err != nil {
				workbook, worksheet := getBookAndSheet(msger)
				issues = append(issues, &Issue{
					Kind:      IssueKindLoad,
					Message:   fmt.Sprintf("process after load all failed: %s", err.Error()),
					Workbook:  workbook,
					Worksheet: worksheet,
				})
				log.Infof("--- FAIL: %v%v ProcessAfterLoadAll", name, loadType)
			}
		}
	}
	return checkers, issues
}

// getBookAndSheet resolves workbook/worksheet options from a messager's
// underlying protobuf message descriptor. Returns nil, nil when the messager
// has no message data; missing extensions yield nil options, which
// Issue.String handles.
func getBookAndSheet(msger tableau.Messager) (*tableaupb.WorkbookOptions, *tableaupb.WorksheetOptions) {
	if msger == nil {
		return nil, nil
	}
	msg := msger.Message()
	if msg == nil {
		return nil, nil
	}
	md := msg.ProtoReflect().Descriptor()
	worksheet, _ := proto.GetExtension(md.Options(), tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	workbook, _ := proto.GetExtension(md.ParentFile().Options(), tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	return workbook, worksheet
}

func (h *Hub) check(breakFailedCount int) []*Issue {
	issues := make([]*Issue, 0, len(h.checkers))
	for _, name := range slices.Sorted(maps.Keys(h.checkers)) {
		c := h.checkers[name]
		log.Infof("=== RUN   %v", name)
		err := c.Check(h.Hub)
		if err != nil {
			workbook, worksheet := getBookAndSheet(c)
			log.Errorf("--- FAIL: workbook %s, worksheet %s", workbook.GetName(), worksheet.GetName())
			issues = append(issues, &Issue{
				Kind:      IssueKindCheck,
				Message:   fmt.Sprintf("custom check failed: %+v", err),
				Workbook:  workbook,
				Worksheet: worksheet,
			})
		} else {
			log.Infof("--- PASS: %v", name)
		}
		if breakFailedCount > 0 && len(issues) >= breakFailedCount {
			break
		}
	}
	return issues
}

func (h *Hub) checkCompatibility(newHub *tableau.Hub, breakFailedCount int) []*Issue {
	issues := make([]*Issue, 0, len(h.checkers))
	for _, name := range slices.Sorted(maps.Keys(h.checkers)) {
		c := h.checkers[name]
		if h.GetMessager(name) == nil || newHub.GetMessager(name) == nil {
			log.Infof("=== SKIP  %v", name)
			continue
		}
		log.Infof("=== RUN   %v", name)
		err := c.CheckCompatibility(h.Hub, newHub)
		if err != nil {
			workbook, worksheet := getBookAndSheet(c)
			log.Errorf("--- FAIL: workbook %s, worksheet %s", workbook.GetName(), worksheet.GetName())
			issues = append(issues, &Issue{
				Kind:      IssueKindCompatibility,
				Message:   fmt.Sprintf("custom check failed: %+v", err),
				Workbook:  workbook,
				Worksheet: worksheet,
			})
		} else {
			log.Infof("--- PASS: %v", name)
		}
		if breakFailedCount > 0 && len(issues) >= breakFailedCount {
			break
		}
	}
	return issues
}

func (h *Hub) Check(dir string, format format.Format, options ...Option) error {
	opts := ParseOptions(options...)
	checkers, loadIssues := h.load(loadTypeDefault, dir, format, opts.LoadOptions...)
	if len(loadIssues) > 0 {
		return &Error{Issues: loadIssues, format: opts.ErrorFormat}
	}
	h.checkers = checkers
	checkIssues := h.check(opts.BreakFailedCount)
	if len(checkIssues) > 0 {
		return &Error{Issues: checkIssues, format: opts.ErrorFormat}
	}
	return nil
}

func (h *Hub) CheckCompatibility(dir, newDir string, format format.Format, options ...Option) error {
	opts := ParseOptions(options...)
	// Load new config first; keep its messager map on newHub.
	_, newLoadIssues := h.load(loadTypeNew, newDir, format, opts.LoadOptions...)
	if len(newLoadIssues) > 0 && !opts.SkipLoadErrors {
		return &Error{Issues: newLoadIssues, format: opts.ErrorFormat}
	}
	newHub := tableau.NewHub()
	newHub.SetMessagerMap(h.GetMessagerMap())
	// Load old config into this hub. Compatibility checkers must own the old
	// messager data, so assign the old checkers explicitly after this load.
	oldCheckers, oldLoadIssues := h.load(loadTypeOld, dir, format, opts.LoadOptions...)
	if len(oldLoadIssues) > 0 && !opts.SkipLoadErrors {
		return &Error{Issues: append(newLoadIssues, oldLoadIssues...), format: opts.ErrorFormat}
	}
	h.checkers = oldCheckers
	compatIssues := h.checkCompatibility(newHub, opts.BreakFailedCount)
	allIssues := append(append(newLoadIssues, oldLoadIssues...), compatIssues...)
	if len(allIssues) > 0 {
		return &Error{Issues: allIssues, format: opts.ErrorFormat}
	}
	return nil
}

type Options struct {
	// BreakFailedCount breaks the check loop once the number of failed checks
	// reaches this value. Values <= 0 mean never break (run all checkers).
	//
	// Default: 1.
	BreakFailedCount int
	// ProtoPackage is retained for API compatibility. Workbook/worksheet
	// metadata is now resolved from each messager's protobuf descriptor, so
	// this field is unused by the hub runtime.
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
	// ErrorFormat controls how issues are formatted when the error is printed.
	// Default: ErrorFormatText.
	ErrorFormat ErrorFormat
}

// Option is the functional option type.
type Option func(*Options)

// BreakFailedCount sets BreakFailedCount option.
// Pass a value <= 0 to run all checkers without early exit.
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

// WithErrorFormat sets the ErrorFormat used to print the returned error.
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
	opts := newDefault()
	for _, setter := range setters {
		setter(opts)
	}
	return opts
}
