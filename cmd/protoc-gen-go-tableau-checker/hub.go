package main

import (
	"path/filepath"

	"google.golang.org/protobuf/compiler/protogen"
)

// generateHub generates related hub files.
func generateHub(gen *protogen.Plugin) {
	filename := filepath.Join("hub." + checkExt + ".go")
	g := gen.NewGeneratedFile(filename, "")
	generateCommonHeader(gen, g)
	g.P()
	g.P("package ", *pkg)
	g.P("import (")
	g.P("tableau ", loaderImportPath)
	g.P()
	g.P(staticHubContent)
	g.P()
}

const staticHubContent = `"fmt"
"os"
"sync"

"github.com/tableauio/tableau/format"
)

type Hub struct {
*tableau.Hub
checkerMap tableau.MessagerMap
}

var hubSingleton *Hub
var once sync.Once

// GetHub return the singleton of Hub
func GetHub() *Hub {
once.Do(func() {
	// new instance
	hubSingleton = &Hub{
		Hub:        tableau.NewHub(),
		checkerMap: tableau.MessagerMap{},
	}
})
return hubSingleton
}

func (h *Hub) Register(msger tableau.Messager) error {
h.checkerMap[msger.Messager().Name()] = msger
return nil
}

func (h *Hub) Load(dir string, filter tableau.Filter, format format.Format) error {
configMap := h.NewMessagerMap(filter)
for name, msger := range h.checkerMap {
	// replace with custom checker
	configMap[name] = msger.Messager()
}
for name, msger := range configMap {
	fmt.Println("=== LOAD  " + name)
	if err := msger.Load(dir, format); err != nil {
		return fmt.Errorf("failed to load %v: %v", name, err)
	}
	fmt.Println("--- DONE: " + name)
}
h.SetMessagerMap(configMap)
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
func register(msger tableau.Messager) {
GetHub().Register(msger)
}`
