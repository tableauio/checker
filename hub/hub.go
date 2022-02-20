package hub

import (
	"fmt"
	"os"
	"sync"

	"github.com/tableauio/checker/protoconf/tableau"
	"github.com/tableauio/tableau/format"
)

type Checker interface {
	Check() error
	Messager() tableau.Messager
}

type CheckerMap = map[string]Checker

type Hub struct {
	*tableau.Hub
	checkerMap CheckerMap
}

var hubSingleton *Hub
var once sync.Once

// GetHub return the singleton of Hub
func GetHub() *Hub {
	once.Do(func() {
		// new instance
		hubSingleton = &Hub{
			Hub:        tableau.NewHub(),
			checkerMap: CheckerMap{},
		}
	})
	return hubSingleton
}

func (h *Hub) Register(checker Checker) {
	h.checkerMap[checker.Messager().Name()] = checker
}

func (h *Hub) Load(dir string, filter tableau.Filter, format format.Format) error {
	configMap := tableau.ConfigMap{}
	for name, checker := range h.checkerMap {
		if err := checker.Messager().Load(dir, format); err != nil {
			return fmt.Errorf("failed to load %v: %v", name, err)
		}
		fmt.Println("load successfully: " + name)
		configMap[name] = checker.Messager()
	}
	h.SetConfigMap(configMap)
	return nil
}

const breakFailedCount = 1

func (h *Hub) Check() {
	failedCount := 0
	for name, checker := range h.checkerMap {
		if err := checker.Check(); err != nil {
			fmt.Printf("check failed: %v, %+v", name, err)
			failedCount++
		}
		if failedCount != 0 && failedCount >= breakFailedCount {
			break
		}
	}
	os.Exit(failedCount)
}

func Register(checker Checker) {
	GetHub().Register(checker)
}
