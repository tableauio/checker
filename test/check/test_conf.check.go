package check

import (
	"fmt"

	"github.com/tableauio/checker/test/protoconf/tableau"
)

func init() {
	register(&ActivityConf{})
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
