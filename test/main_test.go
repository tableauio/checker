package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/checker/test/check"
	"github.com/tableauio/checker/test/protoconf/tableau"
	tableauapi "github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
)

func TestLoad(t *testing.T) {
	hub := check.NewHub(tableau.Filter(loadOriginFilter))
	err := hub.Check("./non-existent-dir/", format.JSON,
		check.BreakFailedCount(10),
		check.WithLoadOptions(load.IgnoreUnknownFields()))
	require.Error(t, err)
	assert.Empty(t, hub.GetMessagerMap(), "failed loads must not be published")
	serr := tableauapi.Inspect(err)
	require.NotEmpty(t, serr.Details)
	for _, detail := range serr.Details {
		assert.Contains(t, detail.Message, "non-existent-dir")
	}
}

func TestCheck(t *testing.T) {
	err := check.NewHub().Check("./testdata/", format.JSON,
		check.BreakFailedCount(1),
		check.WithLoadOptions(load.IgnoreUnknownFields()))
	require.Error(t, err)
	serr := tableauapi.Inspect(err)
	require.Len(t, serr.Details, 1)
	detail := serr.Details[0]
	assert.Equal(t, "check ActivityConf failed: awardId: 0 not found", detail.Message)
	assert.Nil(t, detail.Source)
	assert.Equal(t, detail.Message, serr.Error())
	data, marshalErr := json.Marshal(serr)
	require.NoError(t, marshalErr)
	assert.JSONEq(t, `{"details":[{"message":"check ActivityConf failed: awardId: 0 not found"}]}`, string(data))
}

func TestCheckCompatibility(t *testing.T) {
	err := check.NewHub().CheckCompatibility("./testdata/", "./testdata1/", format.JSON,
		check.SkipLoadErrors(), check.BreakFailedCount(10),
		check.WithLoadOptions(load.IgnoreUnknownFields()))
	require.Error(t, err)
	serr := tableauapi.Inspect(err)
	var loads, compatibility int
	for _, detail := range serr.Details {
		if strings.HasPrefix(detail.Message, "check compatibility of ") {
			compatibility++
			assert.Contains(t, detail.Message, "ItemConf incompatible:")
			assert.Contains(t, detail.Message, "removed in new version:")
		} else {
			loads++
		}
	}
	assert.Positive(t, loads, "load failures must survive SkipLoadErrors")
	assert.Positive(t, compatibility, "compatibility checks must still run")
}

var loadOriginAllowList = map[string]bool{"ItemConf": true, "ChapterConf": true}

func loadOriginFilter(name string) bool { return loadOriginAllowList[name] }

// TestLoadOriginFromCSV verifies valid inputs load successfully and inspecting
// failures yields one Tableau detail per invalid cell across two sheets.
func TestLoadOriginFromCSV(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		err := check.NewHub(tableau.Filter(loadOriginFilter)).Check("./testdata2/", format.CSV,
			check.BreakFailedCount(10))
		require.NoError(t, err)
	})
	err := check.NewHub(tableau.Filter(loadOriginFilter)).Check("./testdata3/", format.CSV,
		check.BreakFailedCount(10),
		check.WithLoadOptions(load.MaxErrorsPerSheet(5)))
	require.Error(t, err)
	serr := tableauapi.Inspect(err)
	counts := map[string]int{}
	for _, detail := range serr.Details {
		assert.NotContains(t, detail.Message, "[1] error")
		require.NotNil(t, detail.Source)
		require.NotNil(t, detail.Source.Cell)
		assert.NotEmpty(t, detail.Source.Cell.Position)
		counts[detail.Source.Worksheet]++
	}
	assert.GreaterOrEqual(t, counts["ItemConf"], 2)
	assert.GreaterOrEqual(t, counts["ChapterConf"], 2)
}
