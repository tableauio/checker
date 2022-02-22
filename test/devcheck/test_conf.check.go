package devcheck

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/tableauio/checker/test/protoconf/tableau"
)

type ActivityConf struct {
	tableau.ActivityConf
}

func (x *ActivityConf) Check() error {
	fmt.Println("ActivityConf: check")
	return nil
}

type ItemConf struct {
	tableau.ItemConf
}

func (x *ItemConf) Check() error {
	fmt.Printf("ItemConf: check\n %v\n", x.ItemConf.Data())

	conf1 := GetHub().GetItemConf()
	if conf1 == nil {
		return errors.Errorf("ItemConf is nil")
	}

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

func init() {
	// register(&ActivityConf{})
	register(&ItemConf{})
}
