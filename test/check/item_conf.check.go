package check

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/tableauio/checker/test/protoconf/tableau"
)

func init() {
	register(&ItemConf{})
}

type ItemConf struct {
	tableau.ItemConf
}

func (x *ItemConf) Messager() tableau.Messager {
	return &x.ItemConf
}

func (x *ItemConf) Check() error {
	fmt.Println("ItemConf: check")

	conf := GetHub().GetActivityConf()
	if conf == nil {
		return errors.Errorf("ActivityConf is nil")
	}
	chapter, err := conf.Get3(100001, 1, 2)
	if err != nil {
		return errors.WithMessagef(err, "failed to get chapter: Get3(100001, 1, 2)")
	}
	fmt.Printf("ActivityConf: %v\n", chapter)

	return nil
}
