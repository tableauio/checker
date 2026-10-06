package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/checker/test/check"
	"github.com/tableauio/checker/test/protoconf/tableau"
	tableauapi "github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"google.golang.org/protobuf/proto"
)

// Exercise the real merger load path: a valid primary workbook and two
// CSV shards, each with two invalid cells.
func TestLoadShardErrorDetails(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		for _, limit := range []int{1, 5} {
			t.Run(fmt.Sprintf("%s/limit%d", lang, limit), func(t *testing.T) {
				require.NoError(t, tableauapi.SetLang(lang))
				t.Cleanup(func() { require.NoError(t, tableauapi.SetLang("en")) })
				err := check.NewHub(tableau.Filter(func(name string) bool {
					return name == "ThemeConf"
				})).Check("./testdata4/", format.CSV,
					check.WithLoadOptions(load.MaxErrorsPerSheet(limit)),
				)
				require.Error(t, err)
				var checkErr *check.Error
				require.ErrorAs(t, err, &checkErr)
				require.Len(t, checkErr.Issues, 1)
				issue := checkErr.Issues[0]
				require.NotNil(t, issue.Details)
				assert.Equal(t, "Test#*.csv", issue.Workbook.GetName())
				assert.Equal(t, "ThemeConf", issue.Worksheet.GetName())
				assert.NotContains(t, err.Error(), "--- debugging ---")
				assert.NotContains(t, err.Error(), "goroutine")
				var native *tableauapi.Error
				require.ErrorAs(t, err, &native)
				assert.Equal(t, native.Details, issue.Details)
				details := issue.Details
				if limit == 1 {
					require.Len(t, details, 1)
				} else {
					require.Len(t, details, 4)
				}
				seen := make(map[string]bool)
				for _, detail := range details {
					require.NotNil(t, detail.Source)
					require.NotNil(t, detail.Source.Cell)
					book := detail.Source.Workbook
					assert.Contains(t, []string{"Merge1#*.csv", "Merge2#*.csv"}, book)
					assert.Equal(t, "Test#*.csv", detail.Source.PrimaryWorkbook)
					assert.Equal(t, "ThemeConf", detail.Source.Worksheet)
					assert.Equal(t, "ThemeConf", detail.Source.PrimaryWorksheet)
					assert.Contains(t, []string{"B4", "B5"}, detail.Source.Cell.Position)
					assert.Contains(t, []string{"bad-first", "bad-second"}, detail.Source.Cell.Data)
					assert.Equal(t, "E2012", detail.Code)
					assert.Equal(t, "confgen", detail.Module)
					assert.Contains(t, issue.Message, detail.Message)
					seen[fmt.Sprintf("%s/%s", book, detail.Source.Cell.Position)] = true
				}
				if limit > 1 {
					assert.Len(t, seen, 4, "each source cell must retain its own error detail")
				}
				if lang == "zh" {
					assert.Contains(t, issue.Message, "(主工作簿: Test#*.csv)")
					assert.Contains(t, issue.Message, "工作表: ThemeConf")
					assert.Contains(t, issue.Message, "单元格位置: B4")
				} else {
					assert.Contains(t, issue.Message, "(Primary: Test#*.csv)")
					assert.Contains(t, issue.Message, "Worksheet: ThemeConf")
					assert.Contains(t, issue.Message, "DataCellPos: B4")
				}

				// JSON carries typed details for each source error.
				var encoded struct {
					Issues []struct {
						Details json.RawMessage `json:"details"`
					} `json:"issues"`
				}
				text := check.ErrorFormatJSON(checkErr)
				require.NoError(t, json.Unmarshal([]byte(text), &encoded))
				require.Len(t, encoded.Issues, 1)
				want, marshalErr := json.Marshal(issue.Details)
				require.NoError(t, marshalErr)
				assert.JSONEq(t, string(want), string(encoded.Issues[0].Details))
				assert.NotContains(t, text, "\n")

				// Retain the original concrete error through the checker wrapper.
				var cause *tableauapi.Error
				require.True(t, errors.As(err, &cause))
				assert.True(t, errors.Is(err, checkErr.Unwrap()[0]))
			})
		}
	}
}

func TestLoadPreservesCause(t *testing.T) {
	cause := errors.New("custom loader unavailable")
	err := check.NewHub(tableau.Filter(func(name string) bool {
		return name == "ChapterConf"
	})).Check("unused", format.JSON,
		check.WithLoadOptions(load.WithLoadFunc(func(proto.Message, string, format.Format, *load.MessagerOptions) error {
			return fmt.Errorf("load config: %w", cause)
		})),
	)
	require.ErrorIs(t, err, cause)
	var checkErr *check.Error
	require.ErrorAs(t, err, &checkErr)
	require.Len(t, checkErr.Issues, 1)
	assert.Nil(t, checkErr.Issues[0].Details)
	assert.Equal(t, "load failed: load config: custom loader unavailable", checkErr.Issues[0].Message)
}
