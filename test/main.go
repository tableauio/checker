package main

import (
	"fmt"

	"github.com/tableauio/checker/test/check"
	// check "github.com/tableauio/checker/test/devcheck"
	"github.com/tableauio/tableau/format"
)

func main() {
	err := check.GetHub().Load("./testdata/", nil, format.JSON)
	if err != nil {
		fmt.Printf("failed to load: %+v\n", err)
	}
	check.GetHub().Check()
}
