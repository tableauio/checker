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
	"github.com/tableauio/tableau/diagnostic"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"google.golang.org/protobuf/proto"
)

// Exercise the real merger load path: a valid primary workbook and two
// shards, each with two invalid cells. A public rewrite option points the
// existing ThemeConf schema at Excel inputs without mutating its descriptor.
func TestLoadShardDiagnostics(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		for _, limit := range []int{1, 5} {
			t.Run(fmt.Sprintf("%s/limit%d", lang, limit), func(t *testing.T) {
				require.NoError(t, tableauapi.SetLang(lang))
				t.Cleanup(func() { require.NoError(t, tableauapi.SetLang("en")) })
				err := check.NewHub(tableau.Filter(func(name string) bool {
					return name == "ThemeConf"
				})).Check("./testdata4/", format.Excel,
					check.WithLoadOptions(load.MaxErrorsPerSheet(limit),
						load.SubdirRewrites(map[string]string{"Test#*.csv": "Test.xlsx"})),
				)
				require.Error(t, err)
				var checkErr *check.Error
				require.ErrorAs(t, err, &checkErr)
				require.Len(t, checkErr.Issues, 1)
				issue := checkErr.Issues[0]
				require.NotNil(t, issue.Diagnostic)
				assert.Equal(t, "Test#*.csv", issue.Workbook.GetName())
				assert.Equal(t, "ThemeConf", issue.Worksheet.GetName())
				assert.NotContains(t, err.Error(), "--- debugging ---")
				assert.NotContains(t, err.Error(), "goroutine")

				leaves := issue.Diagnostic.Children()
				if limit == 1 {
					require.Empty(t, leaves)
					leaves = []*diagnostic.Desc{issue.Diagnostic}
				} else {
					require.Len(t, leaves, 4)
				}
				seen := make(map[string]bool)
				for _, leaf := range leaves {
					book := leaf.GetValue("BookName")
					assert.Contains(t, []string{"Merge1.xlsx", "Merge2.xlsx"}, book)
					assert.Equal(t, "Test#*.csv", leaf.GetValue("PrimaryBookName"))
					assert.Equal(t, "ThemeConf", leaf.GetValue("SheetName"))
					assert.Equal(t, "ThemeConf", leaf.GetValue("PrimarySheetName"))
					assert.Contains(t, []string{"B4", "B5"}, leaf.GetValue("DataCellPos"))
					assert.Contains(t, []string{"bad-first", "bad-second"}, leaf.GetValue("DataCell"))
					assert.Equal(t, "E2012", leaf.GetValue("ErrCode"))
					assert.Equal(t, "confgen", leaf.GetValue("Module"))
					assert.Contains(t, issue.Message, leaf.String())
					seen[fmt.Sprintf("%s/%s", book, leaf.GetValue("DataCellPos"))] = true
				}
				if limit > 1 {
					assert.Len(t, seen, 4, "each source cell must retain its own diagnostic")
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

				// JSON carries the same descriptor, with separate children rather
				// than a synthetic location assembled from unrelated errors.
				var encoded struct {
					Issues []struct {
						Diagnostic json.RawMessage `json:"diagnostic"`
					} `json:"issues"`
				}
				text := check.ErrorFormatJSON(checkErr)
				require.NoError(t, json.Unmarshal([]byte(text), &encoded))
				require.Len(t, encoded.Issues, 1)
				want, marshalErr := json.Marshal(issue.Diagnostic)
				require.NoError(t, marshalErr)
				assert.JSONEq(t, string(want), string(encoded.Issues[0].Diagnostic))
				assert.NotContains(t, text, "\n")

				// Retain the original concrete error through the checker wrapper.
				var cause interface{ Fields() map[string]any }
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
	assert.Nil(t, checkErr.Issues[0].Diagnostic)
	assert.Equal(t, "load failed: load config: custom loader unavailable", checkErr.Issues[0].Message)
}
