package devcheck

import (
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/tableauio/checker/test/protoconf/tableau"
)

type ActivityConf struct {
	tableau.ActivityConf
}

func (x *ActivityConf) Check(hub *tableau.Hub) error {
	// TODO: implement here.
	return nil
}

func (x *ActivityConf) CheckCompatibility(hub, newHub *tableau.Hub) error {
	// TODO: implement here.
	return nil
}

type ItemConf struct {
	tableau.ItemConf
}

func (x *ItemConf) Check(hub *tableau.Hub) error {
	fmt.Printf("ItemConf: check\n %v\n", x.ItemConf.Data())

	conf1 := hub.GetItemConf()
	if conf1 == nil {
		return errors.Errorf("ItemConf is nil")
	}

	conf := hub.GetActivityConf()
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

func (x *ItemConf) CheckCompatibility(hub, newHub *tableau.Hub) error {
	// TODO: implement here.
	log.Printf("old: %v\n", x.Data())
	log.Printf("new: %v", newHub.GetItemConf().Data())
	return nil
}

func init() {
	// NOTE: This func is auto-generated. DO NOT EDIT.
	register("ActivityConf", func() tableau.Messager {
		return &ActivityConf{}
	})
	register("ItemConf", func() tableau.Messager {
		return &ItemConf{}
	})
}
