package checker

import (
	"fmt"

	"github.com/tableauio/checker/hub"
	"github.com/tableauio/checker/protoconf/tableau"
)

func init() {
	hub.Register(&ItemConf{})
}

type ItemConf struct {
	tableau.ItemConf
}

func (x *ItemConf) Messager() tableau.Messager {
	return &x.ItemConf
}

func (x *ItemConf) Check() error {
	fmt.Println("ItemConf: check")

	conf := hub.GetHub().GetActivityConf()
	if conf == nil {
		panic("ActivityConf is nil")
	}
	chapter, err := conf.Get3(100001, 1, 2)
	if err != nil {
		panic(err)
	}
	fmt.Printf("ActivityConf: %v\n", chapter)

	return nil
}
