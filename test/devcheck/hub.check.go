package devcheck

import (
	"fmt"
	"sync"

	tableau "github.com/tableauio/checker/test/protoconf/tableau"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/log"
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

func (h *Hub) load(dir string, format format.Format, subdirRewrites map[string]string) error {
	msgers := tableau.MessagerMap{}
	for name, msger := range h.filteredCheckerMap {
		log.Log().Infof("=== LOAD  %s", name)
		if err := msger.Messager().Load(dir, format, load.SubdirRewrites(subdirRewrites)); err != nil {
			log.Log().Errorf("--- FAIL: %v", name)
			log.Log().Errorf("%+v", err)
			return errors.WithMessagef(err, "failed to load %v", name)
		}
		log.Log().Infof("--- DONE: %v", name)
		msgers[name] = msger.Messager()
	}
	h.SetMessagerMap(msgers)
	return nil
}

func (h *Hub) check(breakFailedCount int) int {
	failedCount := 0
	for name, checker := range h.filteredCheckerMap {
		log.Log().Infof("=== RUN   %v", name)
		// built-in auto-generated check logic
		err1 := checker.Messager().Check(h.Hub)
		// custom check logic
		err2 := checker.Check(h.Hub)
		if err1 != nil || err2 != nil {
			log.Log().Errorf("--- FAIL: %v", name)
			if err1 != nil {
				log.Log().Errorf("auto check error: %+v", err1)
			}
			if err2 != nil {
				log.Log().Errorf("custom check error: %+v", err2)
			}
			failedCount++
		} else {
			log.Log().Infof("--- PASS: %v", name)
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
	err := h.load(dir, format, opts.SubdirRewrites)
	if err != nil {
		return err
	}
	// check
	failedCount := h.check(opts.BreakFailedCount)
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
	SubdirRewrites   map[string]string
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
