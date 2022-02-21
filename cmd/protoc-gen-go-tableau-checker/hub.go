package main

import (
	"path/filepath"

	"google.golang.org/protobuf/compiler/protogen"
)

// generateHub generates related hub files.
func generateHub(gen *protogen.Plugin) {
	filename := filepath.Join(*pkg, "hub."+pcExt+".go")
	g := gen.NewGeneratedFile(filename, "")
	generateCommonHeader(gen, g)
	g.P()
	g.P("package ", *pkg)
	g.P("import (")
	g.P(loaderImportPath)
	g.P()
	g.P(staticHubContent)
	g.P()
}

const staticHubContent = `
	"fmt"
	"os"
	"sync"

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
		fmt.Println("=== LOAD  " + name)
		if err := checker.Messager().Load(dir, format); err != nil {
			return fmt.Errorf("failed to load %v: %v", name, err)
		}
		fmt.Println("--- DONE: " + name)
		configMap[name] = checker.Messager()
	}
	h.SetConfigMap(configMap)
	fmt.Println()
	return nil
}

const breakFailedCount = 1

func (h *Hub) Check() {
	failedCount := 0
	for name, checker := range h.checkerMap {
		fmt.Printf("=== RUN   %v\n", name)
		if err := checker.Check(); err != nil {
			fmt.Printf("--- FAIL: %v\n", name)
			fmt.Printf("    %+v\n", err)
			failedCount++
		} else {
			fmt.Printf("--- PASS: %v\n", name)
		}
		if failedCount != 0 && failedCount >= breakFailedCount {
			break
		}
	}
	os.Exit(failedCount)
}

// Syntatic sugar for Hub's register
func register(checker Checker) {
	GetHub().Register(checker)
}`
