package main

import (
	"github.com/tableauio/checker/hub"
	_ "github.com/tableauio/checker/hub/checker"
	"github.com/tableauio/tableau/format"
)

func main() {
	err := hub.GetHub().Load("./testdata/", nil, format.JSON)
	if err != nil {
		panic(err)
	}
	hub.GetHub().Check(1)
}
