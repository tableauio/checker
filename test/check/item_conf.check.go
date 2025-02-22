// Code generated by protoc-gen-go-tableau-checker.
// versions:
// - protoc-gen-go-tableau-checker v0.4.0
// - protoc                        v3.17.3
// source: item_conf.proto

package check

import (
	"log"

	"github.com/pkg/errors"
	tableau "github.com/tableauio/checker/test/protoconf/tableau"
)

type ItemConf struct {
	tableau.ItemConf
}

func (x *ItemConf) Check(hub *tableau.Hub) error {
	// TODO: implement here.
	return nil
}

func (x *ItemConf) CheckCompatibility(hub, newHub *tableau.Hub) error {
	// TODO: implement here.
	log.Printf("old: %v\n", x.Data())
	log.Printf("new: %v\n", newHub.GetItemConf().Data())
	return errors.Errorf("id missing: 1")
}

func init() {
	// NOTE: This func is auto-generated. DO NOT EDIT.
	register(func() tableau.Messager {
		return new(ItemConf)
	})
}
