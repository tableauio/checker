// Code generated by protoc-gen-go-tableau-checker.
// versions:
// - protoc-gen-go-tableau-checker v0.4.0
// - protoc                        v3.17.3
// source: test_conf.proto

package check

import (
	tableau "github.com/tableauio/checker/test/protoconf/tableau"
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

type ChapterConf struct {
	tableau.ChapterConf
}

func (x *ChapterConf) Check(hub *tableau.Hub) error {
	// TODO: implement here.
	return nil
}

func (x *ChapterConf) CheckCompatibility(hub, newHub *tableau.Hub) error {
	// TODO: implement here.
	return nil
}

type ThemeConf struct {
	tableau.ThemeConf
}

func (x *ThemeConf) Check(hub *tableau.Hub) error {
	// TODO: implement here.
	return nil
}

func (x *ThemeConf) CheckCompatibility(hub, newHub *tableau.Hub) error {
	// TODO: implement here.
	return nil
}

func init() {
	// NOTE: This func is auto-generated. DO NOT EDIT.
	register(func() tableau.Messager {
		return new(ActivityConf)
	})
	register(func() tableau.Messager {
		return new(ChapterConf)
	})
	register(func() tableau.Messager {
		return new(ThemeConf)
	})
}
