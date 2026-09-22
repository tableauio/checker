package check

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/checker/test/customconf"
	"github.com/tableauio/checker/test/protoconf/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
)

func TestProcessAfterLoadAll_CustomItemConf(t *testing.T) {
	hub := NewHub(tableau.Filter(func(name string) bool {
		switch name {
		case "ItemConf", "CustomItemConf":
			return true
		}
		return false
	}))

	_, issues := hub.load(loadTypeDefault, "../testdata", format.JSON, load.IgnoreUnknownFields())
	require.Empty(t, issues)

	conf := tableau.GetMessager[*customconf.CustomItemConf](hub.GetMessagerMap())
	require.NotNil(t, conf, "CustomItemConf should be present after load + ProcessAfterLoadAll")
	assert.Equal(t, "coin1", conf.GetSpecialItemName())

	book, sheet := getBookAndSheet(conf)
	assert.Nil(t, book)
	assert.Nil(t, sheet)
}
