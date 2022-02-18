package checker

import (
	"fmt"

	"github.com/tableauio/checker/hub"
	"github.com/tableauio/checker/protoconf/tableau"
)

func init() {
	hub.Register(&ActivityConf{})
}

type ActivityConf struct {
	tableau.ActivityConf
}

func (x *ActivityConf) Messager() tableau.Messager {
	return &x.ActivityConf
}

func (x *ActivityConf) Check() error {
	fmt.Println("ActivityConf: check")
	return nil
}
