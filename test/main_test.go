package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/checker/test/check"
	"github.com/tableauio/checker/test/protoconf/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
)

var (
	protoPkg   = "protoconf"
	pathPrefix = ""
)

func Filter(messagerName string) bool {
	fullName := protoreflect.FullName(protoPkg + "." + messagerName)
	mt, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
	if err != nil {
		log.Panicf("failed to find messager %s: %+v", fullName, err)
	}
	fd := mt.Descriptor().ParentFile()
	opts := fd.Options().(*descriptorpb.FileOptions)
	workbook := proto.GetExtension(opts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	return strings.HasPrefix(workbook.Name, pathPrefix)
}

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
		assert.Greater(t, len(checkErr.Issues), 0)
		for _, issue := range checkErr.Issues {
			assert.Equal(t, check.IssueKindLoad, issue.Kind)
			assert.Contains(t, issue.Message, "load failed:")
			assert.NotNil(t, issue.Workbook)
			assert.NotNil(t, issue.Worksheet)
			assert.NotEmpty(t, issue.Workbook.GetName())
			assert.NotEmpty(t, issue.Worksheet.GetName())
		}

		errStr := err.Error()
		assert.Contains(t, errStr, "error: workbook")
		assert.Contains(t, errStr, "worksheet")
		assert.Contains(t, errStr, "load failed:")
		// Each issue should be on its own line in text format.
		assert.Equal(t, len(checkErr.Issues), strings.Count(errStr, "error: workbook"))
	})

	t.Run("JSONFormat", func(t *testing.T) {
		err := run(check.ErrorFormatJSON)
		require.Error(t, err)

		var checkErr *check.CheckError
		require.True(t, errors.As(err, &checkErr))
		assert.Greater(t, len(checkErr.Issues), 0)
		for _, issue := range checkErr.Issues {
			assert.Equal(t, check.IssueKindLoad, issue.Kind)
			assert.Contains(t, issue.Message, "load failed:")
		}

		errStr := err.Error()
		assert.Contains(t, errStr, `"issues"`)
		assert.Contains(t, errStr, `"kind":"load"`)
		assert.Contains(t, errStr, `"load failed:`)
		assert.Contains(t, errStr, `"workbook":`)
		assert.Contains(t, errStr, `"worksheet":`)
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
		assert.Len(t, checkErr.Issues, 1)
		issue := checkErr.Issues[0]
		assert.Equal(t, check.IssueKindCheck, issue.Kind)
		assert.Equal(t, "custom check failed: awardId: 0 not found", issue.Message)
		assert.Equal(t, "Test.xlsx", issue.Workbook.GetName())
		assert.Equal(t, "Activity", issue.Worksheet.GetName())

		errStr := err.Error()
		assert.Equal(t,
			"error: workbook Test.xlsx, worksheet Activity, custom check failed: awardId: 0 not found",
			errStr)
	})

	t.Run("JSONFormat", func(t *testing.T) {
		err := run(check.ErrorFormatJSON)
		require.Error(t, err)

		var checkErr *check.CheckError
		require.True(t, errors.As(err, &checkErr))
		assert.Len(t, checkErr.Issues, 1)
		assert.Equal(t, check.IssueKindCheck, checkErr.Issues[0].Kind)

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

	// classifyIssues groups issues by their kind for further inspection.
	classifyIssues := func(issues []*check.Issue) map[check.IssueKind][]*check.Issue {
		m := make(map[check.IssueKind][]*check.Issue)
		for _, i := range issues {
			m[i.Kind] = append(m[i.Kind], i)
		}
		return m
	}

	t.Run("TextFormat", func(t *testing.T) {
		err := run(check.ErrorFormatText)
		require.Error(t, err)

		var checkErr *check.CheckError
		require.True(t, errors.As(err, &checkErr))
		assert.Greater(t, len(checkErr.Issues), 0)

		grouped := classifyIssues(checkErr.Issues)
		assert.NotEmpty(t, grouped[check.IssueKindLoad], "expected load issues")
		assert.NotEmpty(t, grouped[check.IssueKindCompatibility], "expected compatibility issues")

		// Every load issue must carry the expected message prefix and book/sheet info.
		for _, issue := range grouped[check.IssueKindLoad] {
			assert.Contains(t, issue.Message, "load failed:")
			assert.NotEmpty(t, issue.Workbook.GetName())
			assert.NotEmpty(t, issue.Worksheet.GetName())
		}
		// Every compatibility issue must carry the expected message prefix.
		for _, issue := range grouped[check.IssueKindCompatibility] {
			assert.Contains(t, issue.Message, "custom check failed:")
		}

		errStr := err.Error()
		assert.Contains(t, errStr, "error: workbook Test.xlsx")
		assert.Contains(t, errStr, "load failed:")
		assert.Contains(t, errStr, "custom check failed:")
		// ActivityConf's CheckCompatibility intentionally fails with this message.
		assert.Contains(t, errStr,
			"load ItemConf successfully even it's checker is not registered")
	})

	t.Run("JSONFormat", func(t *testing.T) {
		err := run(check.ErrorFormatJSON)
		require.Error(t, err)

		var checkErr *check.CheckError
		require.True(t, errors.As(err, &checkErr))
		assert.Greater(t, len(checkErr.Issues), 0)

		grouped := classifyIssues(checkErr.Issues)
		assert.NotEmpty(t, grouped[check.IssueKindLoad], "expected load issues")
		assert.NotEmpty(t, grouped[check.IssueKindCompatibility], "expected compatibility issues")

		// Note: cannot use assert.JSONEq here because the number of load issues
		// depends on testdata files present, making the full JSON non-deterministic.
		errStr := err.Error()
		assert.Contains(t, errStr, `"issues"`)
		assert.Contains(t, errStr, `"kind":"load"`)
		assert.Contains(t, errStr, `"kind":"compatibility"`)
		assert.Contains(t, errStr, `"load failed:`)
		assert.Contains(t, errStr, `"custom check failed:`)
		assert.Contains(t, errStr, `"workbook":`)
		assert.Contains(t, errStr, `"worksheet":`)
	})
}
