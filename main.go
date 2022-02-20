package main

import (
	"fmt"

	"github.com/tableauio/checker/hub"
	_ "github.com/tableauio/checker/hub/checker"
	"github.com/tableauio/tableau/format"
)

func main() {
	err := hub.GetHub().Load("./testdata/", nil, format.JSON)
	if err != nil {
		fmt.Printf("failed to load: %+v\n", err)
	}
	hub.GetHub().Check()
}
