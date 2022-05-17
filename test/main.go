package main

import (
	"fmt"
	"os"

	"github.com/tableauio/checker/test/check"
	// check "github.com/tableauio/checker/test/devcheck"
	"github.com/tableauio/tableau/format"
)

func main() {
	errs := check.GetHub().Run("./testdata/", nil, format.JSON)
	for _, err := range errs {
		fmt.Printf("failed to load: %+v\n", err)
	}
	failedCount := len(errs)
	os.Exit(failedCount)
}
