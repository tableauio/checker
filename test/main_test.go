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

// structuredError verifies checks return Tableau's error directly.
func structuredError(t *testing.T, err error) *tableauapi.Error {
	t.Helper()
	require.Error(t, err)
	var serr *tableauapi.Error
	require.ErrorAs(t, err, &serr)
	assert.Same(t, serr, err)
	return serr
}

func TestLoad(t *testing.T) {
	err := check.NewHub().Check("./non-existent-dir/", format.JSON,
		check.BreakFailedCount(10),
		check.WithLoadOptions(load.IgnoreUnknownFields()))
	serr := structuredError(t, err)
	require.NotEmpty(t, serr.Details)
	assert.Contains(t, err.Error(), "Workbook:")
	assert.Contains(t, err.Error(), "Worksheet:")
	for _, detail := range serr.Details {
		assert.Contains(t, detail.Message, "load failed:")
		require.NotNil(t, detail.Source)
		assert.NotEmpty(t, detail.Source.Workbook)
		assert.NotEmpty(t, detail.Source.Worksheet)
	}
}

func TestCheck(t *testing.T) {
	err := check.NewHub().Check("./testdata/", format.JSON,
		check.BreakFailedCount(1),
		check.WithLoadOptions(load.IgnoreUnknownFields()))
	serr := structuredError(t, err)
	require.Len(t, serr.Details, 1)
	detail := serr.Details[0]
	assert.Equal(t, "custom check failed: awardId: 0 not found", detail.Message)
	require.NotNil(t, detail.Source)
	assert.Equal(t, "Test#*.csv", detail.Source.Workbook)
	assert.Equal(t, "Activity", detail.Source.Worksheet)
	assert.Equal(t, "Workbook: Test#*.csv\nWorksheet: Activity\nReason: custom check failed: awardId: 0 not found\n", err.Error())
	data, marshalErr := json.Marshal(err)
	require.NoError(t, marshalErr)
	assert.JSONEq(t, `{"details":[{"message":"custom check failed: awardId: 0 not found","source":{"workbook":"Test#*.csv","worksheet":"Activity"}}]}`, string(data))
}

func TestCheckCompatibility(t *testing.T) {
	err := check.NewHub().CheckCompatibility("./testdata/", "./testdata1/", format.JSON,
		check.SkipLoadErrors(), check.BreakFailedCount(10),
		check.WithLoadOptions(load.IgnoreUnknownFields()))
	serr := structuredError(t, err)
	var loads, compatibility int
	for _, detail := range serr.Details {
		require.NotNil(t, detail.Source)
		assert.NotEmpty(t, detail.Source.Workbook)
		assert.NotEmpty(t, detail.Source.Worksheet)
		if strings.HasPrefix(detail.Message, "load") {
			loads++
		} else if strings.HasPrefix(detail.Message, "compatibility check failed:") {
			compatibility++
			assert.Contains(t, detail.Message, "ItemConf incompatible:")
			assert.Contains(t, detail.Message, "removed in new version:")
		}
	}
	assert.Positive(t, loads, "load failures must survive SkipLoadErrors")
	assert.Positive(t, compatibility, "compatibility checks must still run")
}

var loadOriginAllowList = map[string]bool{"ItemConf": true, "ChapterConf": true}

func loadOriginFilter(name string) bool { return loadOriginAllowList[name] }

// TestLoadOriginFromCSV verifies loading succeeds for valid inputs and returns
// one flat Tableau detail per failing cell for invalid inputs across two sheets.
func TestLoadOriginFromCSV(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		err := check.NewHub(tableau.Filter(loadOriginFilter)).Check("./testdata2/", format.CSV,
			check.BreakFailedCount(10))
		require.NoError(t, err)
	})
	err := check.NewHub(tableau.Filter(loadOriginFilter)).Check("./testdata3/", format.CSV,
		check.BreakFailedCount(10),
		check.WithLoadOptions(load.MaxErrorsPerSheet(5)))
	serr := structuredError(t, err)
	counts := map[string]int{}
	for _, detail := range serr.Details {
		assert.Contains(t, detail.Message, "load failed:")
		assert.NotContains(t, detail.Message, "[1] error")
		require.NotNil(t, detail.Source)
		require.NotNil(t, detail.Source.Cell)
		assert.NotEmpty(t, detail.Source.Cell.Position)
		counts[detail.Source.Worksheet]++
	}
	assert.GreaterOrEqual(t, counts["ItemConf"], 2)
	assert.GreaterOrEqual(t, counts["ChapterConf"], 2)
}
