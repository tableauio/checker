package main

import (
	"log"
	"os"

	"github.com/tableauio/checker/test/check"
	"github.com/tableauio/tableau"

	// check "github.com/tableauio/checker/test/devcheck"
	"github.com/tableauio/tableau/format"
)

func main() {
	tableau.SetLog("INFO", "")
	err := check.GetHub().Run("./testdata/", nil, format.JSON)
	if err != nil {
		log.Printf("%+v\n", err)
	}
	os.Exit(-1)
}
