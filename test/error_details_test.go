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
				serr := tableauapi.Inspect(err)
				assert.NotContains(t, serr.Error(), "--- debugging ---")
				assert.NotContains(t, serr.Error(), "goroutine")
				details := serr.Details
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
					assert.Contains(t, serr.Error(), detail.Message)
					seen[fmt.Sprintf("%s/%s", book, detail.Source.Cell.Position)] = true
				}
				if limit > 1 {
					assert.Len(t, seen, 4, "each source cell must retain its own error detail")
				}
				if lang == "zh" {
					assert.Contains(t, serr.Error(), "(主工作簿: Test#*.csv)")
					assert.Contains(t, serr.Error(), "工作表: ThemeConf")
					assert.Contains(t, serr.Error(), "单元格位置: B4")
				} else {
					assert.Contains(t, serr.Error(), "(Primary: Test#*.csv)")
					assert.Contains(t, serr.Error(), "Worksheet: ThemeConf")
					assert.Contains(t, serr.Error(), "DataCellPos: B4")
				}

				// JSON uses the same flat Tableau details as text output.
				var encoded tableauapi.Error
				data, marshalErr := json.Marshal(serr)
				require.NoError(t, marshalErr)
				require.NoError(t, json.Unmarshal(data, &encoded))
				want, marshalErr := json.Marshal(serr.Details)
				require.NoError(t, marshalErr)
				got, marshalErr := json.Marshal(encoded.Details)
				require.NoError(t, marshalErr)
				assert.JSONEq(t, string(want), string(got))
				assert.NotContains(t, string(data), "\n")
				assert.NotContains(t, string(data), `"issues"`)
				assert.True(t, errors.Is(err, serr.Unwrap()))
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
	serr := tableauapi.Inspect(err)
	require.ErrorIs(t, serr, cause)
	require.Len(t, serr.Details, 1)
	assert.Equal(t, "load failed: load config: custom loader unavailable", serr.Details[0].Message)
	assert.Equal(t, "Test#*.csv", serr.Details[0].Source.Workbook)
	assert.Equal(t, "ChapterConf", serr.Details[0].Source.Worksheet)
}

func TestLoadKeepsSuccessfulMessagers(t *testing.T) {
	cause := errors.New("item config unavailable")
	hub := check.NewHub(tableau.Filter(loadOriginFilter))
	err := hub.Check("unused", format.JSON,
		check.WithLoadOptions(load.WithLoadFunc(func(msg proto.Message, _ string, _ format.Format, _ *load.MessagerOptions) error {
			if string(msg.ProtoReflect().Descriptor().Name()) == "ItemConf" {
				return cause
			}
			return nil
		})),
	)
	require.ErrorIs(t, err, cause)
	assert.NotNil(t, hub.GetMessager("ChapterConf"))
	assert.Nil(t, hub.GetMessager("ItemConf"))
	serr := tableauapi.Inspect(err)
	require.Len(t, serr.Details, 1)
	assert.Equal(t, "ItemConf", serr.Details[0].Source.Worksheet)
}

func TestStructuredFailurePreservesInput(t *testing.T) {
	original := &tableauapi.Error{Details: []*tableauapi.ErrorDetail{{
		Message: "invalid number",
		Source:  &tableauapi.SourceLocation{Workbook: "Shard#*.csv"},
	}}}
	err := check.NewHub(tableau.Filter(func(name string) bool { return name == "ChapterConf" })).Check(
		"unused", format.JSON,
		check.WithLoadOptions(load.WithLoadFunc(func(proto.Message, string, format.Format, *load.MessagerOptions) error {
			return original
		})),
	)
	require.ErrorIs(t, err, original)
	serr := tableauapi.Inspect(err)
	require.Len(t, serr.Details, 1)
	assert.Equal(t, "load failed: invalid number", serr.Details[0].Message)
	assert.Equal(t, "Shard#*.csv", serr.Details[0].Source.Workbook)
	assert.Equal(t, "ChapterConf", serr.Details[0].Source.Worksheet)
	assert.Equal(t, "invalid number", original.Details[0].Message)
	assert.Empty(t, original.Details[0].Source.Worksheet)
}

func TestMixedFailuresPreserveCausesAndInput(t *testing.T) {
	original := &tableauapi.Error{Details: []*tableauapi.ErrorDetail{{
		Code: "E2012", Description: "invalid syntax of numerical value", Message: "invalid number", Module: "confgen",
		Source: &tableauapi.SourceLocation{Workbook: "Shard#*.csv", Worksheet: "ShardSheet", PrimaryWorkbook: "Main#*.csv"},
	}}}
	plain := errors.New("custom loader unavailable")
	err := check.NewHub(tableau.Filter(func(name string) bool { return name == "ChapterConf" })).Check(
		"unused", format.JSON,
		check.WithLoadOptions(load.WithLoadFunc(func(proto.Message, string, format.Format, *load.MessagerOptions) error {
			return errors.Join(original, plain)
		})),
	)
	require.Error(t, err)
	serr := tableauapi.Inspect(err)
	require.Len(t, serr.Details, 2)
	require.ErrorIs(t, err, original)
	require.ErrorIs(t, err, plain)
	require.ErrorIs(t, serr, original)
	require.ErrorIs(t, serr, plain)
	assert.Equal(t, "load failed: invalid number", serr.Details[0].Message)
	assert.Equal(t, "Shard#*.csv", serr.Details[0].Source.Workbook)
	assert.Equal(t, "Main#*.csv", serr.Details[0].Source.PrimaryWorkbook)
	assert.Equal(t, "load failed: custom loader unavailable", serr.Details[1].Message)
	assert.Equal(t, "Test#*.csv", serr.Details[1].Source.Workbook)
	assert.Equal(t, "invalid number", original.Details[0].Message)
	assert.Equal(t, "Shard#*.csv", original.Details[0].Source.Workbook)
}

func TestCustomErrorsLocalized(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {
			require.NoError(t, tableauapi.SetLang(lang))
			t.Cleanup(func() { require.NoError(t, tableauapi.SetLang("en")) })
			err := check.NewHub(tableau.Filter(func(name string) bool { return name == "ChapterConf" })).Check(
				"unused", format.JSON,
				check.WithLoadOptions(load.WithLoadFunc(func(proto.Message, string, format.Format, *load.MessagerOptions) error {
					return errors.New("unavailable")
				})),
			)
			require.Error(t, err)
			serr := tableauapi.Inspect(err)
			require.Len(t, serr.Details, 1)
			if lang == "zh" {
				assert.Contains(t, serr.Error(), "工作簿: Test#*.csv")
				assert.Contains(t, serr.Error(), "工作表: ChapterConf")
				assert.Contains(t, serr.Error(), "错误原因: load failed: unavailable")
			} else {
				assert.Contains(t, serr.Error(), "Workbook: Test#*.csv")
				assert.Contains(t, serr.Error(), "Worksheet: ChapterConf")
				assert.Contains(t, serr.Error(), "Reason: load failed: unavailable")
			}
			assert.NotContains(t, serr.Error(), "error[]")
		})
	}
}
