package check

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tableau "github.com/tableauio/checker/test/protoconf/tableau"
	tableauapi "github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type failingChecker struct {
	tableau.ActivityConf
	failure error
	message proto.Message
}

func (c *failingChecker) Message() proto.Message {
	if c.message != nil {
		return c.message
	}
	return c.ActivityConf.Message()
}

func sourceMessage(t *testing.T, alias string, workbook *tableaupb.WorkbookOptions, worksheet *tableaupb.WorksheetOptions) proto.Message {
	t.Helper()
	fileOptions := &descriptorpb.FileOptions{}
	proto.SetExtension(fileOptions, tableaupb.E_Workbook, workbook)
	messageOptions := &descriptorpb.MessageOptions{}
	proto.SetExtension(messageOptions, tableaupb.E_Worksheet, worksheet)
	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name: proto.String("source.proto"), Syntax: proto.String("proto3"), Package: proto.String("checktest"),
		Dependency: []string{"tableau/protobuf/tableau.proto"}, Options: fileOptions,
		MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String(alias), Options: messageOptions}},
	}, protoregistry.GlobalFiles)
	require.NoError(t, err)
	return dynamicpb.NewMessage(fd.Messages().Get(0))
}

func (c *failingChecker) Check(*tableau.Hub) error { return c.failure }
func (c *failingChecker) CheckCompatibility(*tableau.Hub, *tableau.Hub) error {
	return c.failure
}
func (c *failingChecker) ProcessAfterLoadAll(*tableau.Hub) error { return c.failure }
func (c *failingChecker) Messager() tableau.Messager             { return c }

func TestCustomFailureSource(t *testing.T) {
	for _, phase := range []string{"check", "compatibility", "post-load"} {
		t.Run(phase, func(t *testing.T) {
			cause := errors.New("condition missing")
			c := &failingChecker{failure: cause}
			hub := NewHub(tableau.Filter(func(name string) bool { return name == "ActivityConf" }))
			hub.checkers["ActivityConf"] = c
			hub.SetMessagerMap(tableau.MessagerMap{"ActivityConf": c})
			var failures []error
			switch phase {
			case "check":
				failures = hub.check(0)
			case "compatibility":
				newHub := tableau.NewHub()
				newHub.SetMessagerMap(tableau.MessagerMap{"ActivityConf": c})
				failures = hub.checkCompatibility(newHub, 0)
			case "post-load":
				generators := getRegistrar().Generators
				original := generators["ActivityConf"]
				generators["ActivityConf"] = func() checker { return c }
				t.Cleanup(func() { generators["ActivityConf"] = original })
				_, failures = hub.load(loadTypeDefault, "unused", format.JSON,
					load.WithLoadFunc(func(proto.Message, string, format.Format, *load.MessagerOptions) error { return nil }))
			}
			require.Len(t, failures, 1)
			serr := tableauapi.Inspect(errors.Join(failures...))
			require.ErrorIs(t, serr, cause)
			require.Len(t, serr.Details, 1)
			require.NotNil(t, serr.Details[0].Source)
			assert.Equal(t, "Test#*.csv", serr.Details[0].Source.Workbook)
			assert.Equal(t, "Activity", serr.Details[0].Source.Worksheet)
			assert.Equal(t, "ActivityConf", serr.Details[0].Source.WorksheetAlias)
			assert.Nil(t, serr.Details[0].Source.Cell)
			assert.Equal(t, "condition missing", serr.Details[0].Message)
			assert.Equal(t, "E0005", serr.Details[0].Code)
		})
	}
}

func TestCustomFailureKeepsPreciseSource(t *testing.T) {
	source := &tableauapi.SourceLocation{Workbook: "Shard.xlsx", Worksheet: "SubSheet", WorksheetAlias: "SubConf",
		Cell: &tableauapi.CellLocation{Position: "B4", Data: "invalid"}}
	for _, tt := range []struct {
		name        string
		err         error
		wantMessage string
		wantSource  *tableauapi.SourceLocation
		wantCode    string
	}{
		{
			name:        "structured error",
			wantMessage: "invalid value",
			wantCode:    "E0005",
			wantSource:  source,
			err: &tableauapi.Error{Details: []*tableauapi.ErrorDetail{{
				Message: "invalid value", Source: source,
			}}},
		},
		{
			name:        "metadata error",
			wantMessage: "invalid value",
			wantCode:    "E0005",
			wantSource:  &tableauapi.SourceLocation{Workbook: "Shard.xlsx", Worksheet: "SubSheet", WorksheetAlias: "SubConf"},
			err: tableauapi.WrapKV(errors.New("invalid value"),
				tableauapi.KeyBookName, "Shard.xlsx", tableauapi.KeySheetName, "SubSheet", tableauapi.KeySheetAlias, "SubConf"),
		},
		{
			name:        "coded error",
			wantMessage: "invalid value",
			wantCode:    "E2012",
			wantSource:  source,
			err: &tableauapi.Error{Details: []*tableauapi.ErrorDetail{{
				Code: "E2012", Description: "invalid syntax of numerical value", Module: "confgen",
				Message: "invalid value", Source: source,
			}}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hub := NewHub()
			hub.checkers["ActivityConf"] = &failingChecker{failure: tt.err}
			serr := tableauapi.Inspect(errors.Join(hub.check(0)...))
			require.ErrorIs(t, serr, tt.err)
			require.Len(t, serr.Details, 1)
			assert.Equal(t, tt.wantSource, serr.Details[0].Source)
			assert.Equal(t, tt.wantMessage, serr.Details[0].Message)
			assert.Equal(t, tt.wantCode, serr.Details[0].Code)
			assert.Equal(t, tt.wantSource, tableauapi.Inspect(tt.err).Details[0].Source)
		})
	}
}

func TestCustomFailureSchemaContext(t *testing.T) {
	for _, mode := range []string{"Merger", "Scatter"} {
		for _, phase := range []string{"check", "compatibility", "post-load"} {
			t.Run(mode+"/"+phase, func(t *testing.T) {
				patterns := []string{"Season*.csv#Daily*", "BattlePass*.csv#Task*"}
				worksheet := &tableaupb.WorksheetOptions{Name: "Task"}
				if mode == "Merger" {
					worksheet.Merger = patterns
				} else {
					worksheet.Scatter = patterns
				}
				cause := errors.New("condition missing")
				c := &failingChecker{failure: cause, message: sourceMessage(t, "TaskConf",
					&tableaupb.WorkbookOptions{Name: "Task#*.csv", Alias: "Tasks"}, worksheet)}
				hub := NewHub(tableau.Filter(func(name string) bool { return name == "ActivityConf" }))
				hub.checkers["ActivityConf"] = c
				hub.SetMessagerMap(tableau.MessagerMap{"ActivityConf": c})
				var failures []error
				switch phase {
				case "check":
					failures = hub.check(0)
				case "compatibility":
					newHub := tableau.NewHub()
					newHub.SetMessagerMap(tableau.MessagerMap{"ActivityConf": c})
					failures = hub.checkCompatibility(newHub, 0)
				case "post-load":
					generators := getRegistrar().Generators
					original := generators["ActivityConf"]
					generators["ActivityConf"] = func() checker { return c }
					t.Cleanup(func() { generators["ActivityConf"] = original })
					_, failures = hub.load(loadTypeDefault, "unused", format.JSON,
						load.WithLoadFunc(func(proto.Message, string, format.Format, *load.MessagerOptions) error { return nil }))
				}
				require.Len(t, failures, 1)
				serr := tableauapi.Inspect(errors.Join(failures...))
				require.ErrorIs(t, serr, cause)
				require.Len(t, serr.Details, 1)
				source := serr.Details[0].Source
				require.NotNil(t, source)
				assert.Equal(t, "Tasks", source.WorkbookAlias)
				assert.Equal(t, "TaskConf", source.WorksheetAlias)
				assert.Equal(t, "error[E0005]: custom check failed\nWorkbook: Task#*.csv (Alias: Tasks)\nWorksheet: Task (Alias: TaskConf, "+mode+": [Season*.csv#Daily*, BattlePass*.csv#Task*])\nReason: condition missing\n", serr.Error())
				if mode == "Merger" {
					assert.Equal(t, patterns, source.Merger)
					assert.Empty(t, source.Scatter)
				} else {
					assert.Equal(t, patterns, source.Scatter)
					assert.Empty(t, source.Merger)
				}
				encoded, err := json.Marshal(serr)
				require.NoError(t, err)
				var decoded tableauapi.Error
				require.NoError(t, json.Unmarshal(encoded, &decoded))
				assert.Equal(t, source, decoded.Details[0].Source)
				assert.Equal(t, serr.Error(), decoded.Error())
			})
		}
	}
}
