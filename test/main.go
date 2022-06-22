package main

import (
	"fmt"
	"os"

	"github.com/tableauio/checker/test/check"

	// check "github.com/tableauio/checker/test/devcheck"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
)

func main() {
	log.Init(&options.LogOption{
		Mode:     "FULL",
		Level:    "INFO",
		Filename: "_logs/checker.log",
		Sink:     "MULTI",
	})
	err := check.GetHub().Run("./testdata/", nil, format.JSON)
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(-1)
	}
}
