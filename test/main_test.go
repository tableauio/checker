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

		var checkErr *check.Error
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

		var checkErr *check.Error
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

		var checkErr *check.Error
		require.True(t, errors.As(err, &checkErr))
		assert.Len(t, checkErr.Issues, 1)
		issue := checkErr.Issues[0]
		assert.Equal(t, check.IssueKindCheck, issue.Kind)
		assert.Equal(t, "custom check failed: awardId: 0 not found", issue.Message)
		assert.Equal(t, "Test#*.csv", issue.Workbook.GetName())
		assert.Equal(t, "Activity", issue.Worksheet.GetName())

		errStr := err.Error()
		assert.Equal(t,
			"error: workbook Test#*.csv, worksheet Activity, custom check failed: awardId: 0 not found",
			errStr)
	})

	t.Run("JSONFormat", func(t *testing.T) {
		err := run(check.ErrorFormatJSON)
		require.Error(t, err)

		var checkErr *check.Error
		require.True(t, errors.As(err, &checkErr))
		assert.Len(t, checkErr.Issues, 1)
		assert.Equal(t, check.IssueKindCheck, checkErr.Issues[0].Kind)

		// Workbook/Worksheet use protojson field names (camelCase).
		assert.JSONEq(t, `{
			"issues": [
				{
					"kind": "check",
					"message": "custom check failed: awardId: 0 not found",
					"workbook": {"name": "Test#*.csv"},
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

		var checkErr *check.Error
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
		assert.Contains(t, errStr, "error: workbook Test#*.csv")
		assert.Contains(t, errStr, "load failed:")
		assert.Contains(t, errStr, "custom check failed:")
		// ActivityConf's CheckCompatibility reports ItemConf entries that
		// existed in the old snapshot but were removed in the new one.
		assert.Contains(t, errStr, "ItemConf incompatible:")
		assert.Contains(t, errStr, "removed in new version:")
	})

	t.Run("JSONFormat", func(t *testing.T) {
		err := run(check.ErrorFormatJSON)
		require.Error(t, err)

		var checkErr *check.Error
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

// loadOriginAllowList limits Hub loading to messagers whose CSV layout is
// trivial (single vertical map, no refer / merger / scatter), so that the
// loadOrigin-from-CSV path can be exercised by testdata2/testdata3 without
// pulling in the more complex ActivityConf / ThemeConf layouts.
var loadOriginAllowList = map[string]bool{
	"ItemConf":    true,
	"ChapterConf": true,
}

func loadOriginFilter(name string) bool {
	return loadOriginAllowList[name]
}

// TestLoadOriginFromCSV verifies that the checker can drive tableau's
// loadOrigin path against real CSV inputs.
//
// The allow-listed messagers (ItemConf + ChapterConf) belong to two
// separate workbooks ("Item#*.csv" and "Test#*.csv") and use the
// simplest possible layouts (single vertical map of scalars), so the
// success scenario exercises the end-to-end CSV loading pipeline
// without depending on cross-sheet refer / merger / scatter.
func TestLoadOriginFromCSV(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		err := check.NewHub(tableau.Filter(loadOriginFilter)).Check(
			"./testdata2/", format.CSV,
			check.BreakFailedCount(10),
			check.WithErrorFormat(check.ErrorFormatText),
		)
		require.NoError(t, err, "expected loadOrigin from valid CSV inputs to succeed")
	})

	// runFail loads testdata3/, where every allow-listed messager's CSV
	// contains multiple cell-level errors. confgen's per-sheet child
	// collector aggregates those errors into a single multi-line wrapped
	// error per sheet, which the checker surfaces as one Issue per sheet.
	//
	// MaxErrorsPerSheet is bumped above the default fail-fast cap of 1
	// so that loadOrigin's top-level collector lets confgen actually
	// aggregate multiple row-level errors before bailing out.
	runFail := func(t *testing.T, ef check.ErrorFormat) (*check.Error, string) {
		t.Helper()
		err := check.NewHub(tableau.Filter(loadOriginFilter)).Check(
			"./testdata3/", format.CSV,
			check.BreakFailedCount(10),
			check.WithErrorFormat(ef),
			check.WithLoadOptions(
				load.MaxErrorsPerSheet(5),
			),
		)
		require.Error(t, err)

		var checkErr *check.Error
		require.True(t, errors.As(err, &checkErr))
		return checkErr, err.Error()
	}

	t.Run("Failure_TextFormat", func(t *testing.T) {
		checkErr, errStr := runFail(t, check.ErrorFormatText)

		// One issue per allow-listed messager (one CSV / sheet each).
		assert.Len(t, checkErr.Issues, len(loadOriginAllowList),
			"expected exactly one load issue per allow-listed messager")

		seen := map[string]bool{}
		for _, issue := range checkErr.Issues {
			assert.Equal(t, check.IssueKindLoad, issue.Kind)
			assert.Contains(t, issue.Message, "load failed:")
			assert.NotEmpty(t, issue.Workbook.GetName())
			assert.NotEmpty(t, issue.Worksheet.GetName())
			// Each sheet had >=2 cell-level errors → confgen aggregates them
			// with [1], [2], ... prefixes inside the wrapped error message.
			assert.Contains(t, issue.Message, "[1] error",
				"expected first aggregated sub-error in %q", issue.Worksheet.GetName())
			assert.Contains(t, issue.Message, "[2] error",
				"expected second aggregated sub-error in %q", issue.Worksheet.GetName())
			seen[issue.Worksheet.GetName()] = true
		}
		assert.True(t, seen["ItemConf"], "expected an issue for ItemConf")
		assert.True(t, seen["ChapterConf"], "expected an issue for ChapterConf")

		// Each issue is rendered on its own "error: workbook ..." line in
		// text format. The aggregated multi-line load error sits inside
		// the message portion, separated by '\n'.
		assert.Equal(t, len(checkErr.Issues), strings.Count(errStr, "error: workbook"))
		assert.Contains(t, errStr, "error: workbook Item#*.csv, worksheet ItemConf, load failed:")
		assert.Contains(t, errStr, "error: workbook Test#*.csv, worksheet ChapterConf, load failed:")

		t.Logf("\n----- TextFormat output (%d issues) -----\n%s\n----- end -----",
			len(checkErr.Issues), errStr)
	})

	t.Run("Failure_JSONFormat", func(t *testing.T) {
		checkErr, errStr := runFail(t, check.ErrorFormatJSON)

		assert.Len(t, checkErr.Issues, len(loadOriginAllowList))

		// JSON output must remain valid even when issue messages contain
		// embedded newlines from the aggregated load error: every '\n'
		// inside a "message" field has to be escaped as "\n".
		assert.Contains(t, errStr, `"issues"`)
		assert.Contains(t, errStr, `"kind":"load"`)
		assert.Contains(t, errStr, `"load failed:`)
		assert.Contains(t, errStr, `"workbook":`)
		assert.Contains(t, errStr, `"worksheet":`)
		// Aggregated sub-error markers (with newlines escaped).
		assert.Contains(t, errStr, `[1] error`)
		assert.Contains(t, errStr, `[2] error`)
		// The top-level JSON object must be a single line: no raw newlines
		// must leak into the rendered JSON document.
		assert.NotContains(t, errStr, "\n",
			"json output must not contain raw newlines")

		t.Logf("\n----- JSONFormat output (%d issues) -----\n%s\n----- end -----",
			len(checkErr.Issues), errStr)
	})
}
