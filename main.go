package main

import (
	"github.com/tableauio/checker/hub"
	_ "github.com/tableauio/checker/hub/checker"
	"github.com/tableauio/tableau/options"
)

func main() {
	err := hub.GetHub().Load("./testdata/", nil, options.JSON)
	if err != nil {
		panic(err)
	}
	hub.GetHub().Check(1)
}
