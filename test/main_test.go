package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/checker/test/check"
	"github.com/tableauio/checker/test/protoconf/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
)

func TestLoad(t *testing.T) {
	run := func(ef check.ErrorFormat) error {
		return check.NewHub(tableau.Filter(Filter)).Check("./non-existent-dir/", format.JSON,
			check.BreakFailedCount(10),
			check.WithErrorFormat(ef),
			check.WithLoadOptions(load.IgnoreUnknownFields()),
		)
	}

	t.Run("TextFormat", func(t *testing.T) {
		err := run(check.ErrorFormatText)
		require.Error(t, err)

		var checkErr *check.CheckError
		require.True(t, errors.As(err, &checkErr))
		assert.Greater(t, len(checkErr.Result.Issues), 0)
		for _, issue := range checkErr.Result.Issues {
			assert.Equal(t, check.IssueKindLoad, issue.Kind)
		}

		errStr := err.Error()
		assert.Contains(t, errStr, "load failed:")
	})

	t.Run("JSONFormat", func(t *testing.T) {
		err := run(check.ErrorFormatJSON)
		require.Error(t, err)

		var checkErr *check.CheckError
		require.True(t, errors.As(err, &checkErr))
		assert.Greater(t, len(checkErr.Result.Issues), 0)
		for _, issue := range checkErr.Result.Issues {
			assert.Equal(t, check.IssueKindLoad, issue.Kind)
		}

		errStr := err.Error()
		assert.Contains(t, errStr, `"issues"`)
		assert.Contains(t, errStr, `"kind":"load"`)
		assert.Contains(t, errStr, `"load failed:`)
	})
}

func TestCheck(t *testing.T) {
	run := func(ef check.ErrorFormat) error {
		return check.NewHub(tableau.Filter(Filter)).Check("./testdata/", format.JSON,
			check.BreakFailedCount(1),
			check.WithErrorFormat(ef),
			check.WithLoadOptions(load.IgnoreUnknownFields()),
		)
	}

	t.Run("TextFormat", func(t *testing.T) {
		err := run(check.ErrorFormatText)
		require.Error(t, err)

		var checkErr *check.CheckError
		require.True(t, errors.As(err, &checkErr))
		assert.Len(t, checkErr.Result.Issues, 1)
		assert.Equal(t, check.IssueKindCheck, checkErr.Result.Issues[0].Kind)

		errStr := err.Error()
		assert.Contains(t, errStr, "error: workbook Test.xlsx")
		assert.Contains(t, errStr, "worksheet Activity")
		assert.Contains(t, errStr, "custom check failed: awardId: 0 not found")
	})

	t.Run("JSONFormat", func(t *testing.T) {
		err := run(check.ErrorFormatJSON)
		require.Error(t, err)

		var checkErr *check.CheckError
		require.True(t, errors.As(err, &checkErr))
		assert.Len(t, checkErr.Result.Issues, 1)
		assert.Equal(t, check.IssueKindCheck, checkErr.Result.Issues[0].Kind)

		// Workbook/Worksheet use protojson field names (camelCase).
		assert.JSONEq(t, `{
			"issues": [
				{
					"kind": "check",
					"message": "custom check failed: awardId: 0 not found",
					"workbook": {"name": "Test.xlsx"},
					"worksheet": {
						"name": "Activity",
						"orderedMap": true,
						"index": ["ChapterID", "ChapterName@NamedChapter", "SectionItemId@Award"]
					}
				}
			]
		}`, err.Error())
	})
}

func TestCheckCompatibility(t *testing.T) {
	run := func(ef check.ErrorFormat) error {
		return check.NewHub(tableau.Filter(Filter)).CheckCompatibility("./testdata/", "./testdata1/", format.JSON,
			check.SkipLoadErrors(),
			check.BreakFailedCount(10),
			check.WithErrorFormat(ef),
			check.WithLoadOptions(load.IgnoreUnknownFields()),
		)
	}

	t.Run("TextFormat", func(t *testing.T) {
		err := run(check.ErrorFormatText)
		require.Error(t, err)

		var checkErr *check.CheckError
		require.True(t, errors.As(err, &checkErr))
		assert.Greater(t, len(checkErr.Result.Issues), 0)

		// Should contain both load and compatibility issues
		kindSet := make(map[check.IssueKind]bool)
		for _, issue := range checkErr.Result.Issues {
			kindSet[issue.Kind] = true
		}
		assert.True(t, kindSet[check.IssueKindLoad], "expected load issues")
		assert.True(t, kindSet[check.IssueKindCompatibility], "expected compatibility issues")

		errStr := err.Error()
		assert.Contains(t, errStr, "error: workbook Test.xlsx")
	})

	t.Run("JSONFormat", func(t *testing.T) {
		err := run(check.ErrorFormatJSON)
		require.Error(t, err)

		var checkErr *check.CheckError
		require.True(t, errors.As(err, &checkErr))
		assert.Greater(t, len(checkErr.Result.Issues), 0)

		// Should contain both load and compatibility issues
		kindSet := make(map[check.IssueKind]bool)
		for _, issue := range checkErr.Result.Issues {
			kindSet[issue.Kind] = true
		}
		assert.True(t, kindSet[check.IssueKindLoad], "expected load issues")
		assert.True(t, kindSet[check.IssueKindCompatibility], "expected compatibility issues")

		// Note: cannot use assert.JSONEq here because the number of load issues
		// depends on testdata files present, making the full JSON non-deterministic.
		errStr := err.Error()
		assert.Contains(t, errStr, `"issues"`)
		assert.Contains(t, errStr, `"kind":"load"`)
		assert.Contains(t, errStr, `"kind":"compatibility"`)
	})
}
