package check

import (
	"fmt"

	tableau "github.com/tableauio/checker/test/protoconf/tableau"
)

func (x *ActivityConf) Check(hub *tableau.Hub) error {
	// TODO: implement here.
	return nil
}

func (x *ActivityConf) CheckCompatibility(hub, newHub *tableau.Hub) error {
	return fmt.Errorf("load ItemConf successfully even it's checker is not registered\n\nItemConf(old): %v\n\nItemConf(new): %v",
		hub.GetItemConf().Data(), newHub.GetItemConf().Data())
}

func (x *ChapterConf) Check(hub *tableau.Hub) error {
	// TODO: implement here.
	return nil
}

func (x *ChapterConf) CheckCompatibility(hub, newHub *tableau.Hub) error {
	return fmt.Errorf("should not reach here since ChapterConf is not successfully loaded")
}

func (x *ThemeConf) Check(hub *tableau.Hub) error {
	// TODO: implement here.
	return nil
}

func (x *ThemeConf) CheckCompatibility(hub, newHub *tableau.Hub) error {
	return fmt.Errorf("should not reach here since ThemeConf is not successfully loaded")
}
