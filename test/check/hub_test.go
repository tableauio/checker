package check

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tableau "github.com/tableauio/checker/test/protoconf/tableau"
	tableauapi "github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"google.golang.org/protobuf/proto"
)

type failingChecker struct {
	tableau.ActivityConf
	failure error
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
			assert.Nil(t, serr.Details[0].Source.Cell)
		})
	}
}

func TestCustomFailureKeepsPreciseSource(t *testing.T) {
	source := &tableauapi.SourceLocation{Workbook: "Shard.xlsx", Worksheet: "SubSheet",
		Cell: &tableauapi.CellLocation{Position: "B4", Data: "invalid"}}
	for _, tt := range []struct {
		name        string
		err         error
		wantMessage string
	}{
		{
			name:        "structured error",
			wantMessage: "invalid value",
			err: &tableauapi.Error{Details: []*tableauapi.ErrorDetail{{
				Message: "invalid value", Source: source,
			}}},
		},
		{
			name:        "metadata error",
			wantMessage: "check ActivityConf failed: invalid value",
			err: tableauapi.NewKV("invalid value",
				tableauapi.KeyBookName, "Shard.xlsx", tableauapi.KeySheetName, "SubSheet",
				tableauapi.KeyDataCellPos, "B4", tableauapi.KeyDataCell, "invalid"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hub := NewHub()
			hub.checkers["ActivityConf"] = &failingChecker{failure: tt.err}
			serr := tableauapi.Inspect(errors.Join(hub.check(0)...))
			require.ErrorIs(t, serr, tt.err)
			require.Len(t, serr.Details, 1)
			assert.Equal(t, source, serr.Details[0].Source)
			assert.Equal(t, tt.wantMessage, serr.Details[0].Message)
			assert.Equal(t, source, tableauapi.Inspect(tt.err).Details[0].Source)
		})
	}
}
