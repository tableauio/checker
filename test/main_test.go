package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	require.NoError(t, tableauapi.SetLang("en"))
	err := check.NewHub().Check("./testdata/", format.JSON,
		check.BreakFailedCount(1),
		check.WithLoadOptions(load.IgnoreUnknownFields()))
	require.Error(t, err)
	serr := tableauapi.Inspect(err)
	require.Len(t, serr.Details, 1)
	detail := serr.Details[0]
	assert.Equal(t, "awardId: 0 not found", detail.Message)
	require.NotNil(t, detail.Source)
	assert.Equal(t, "Test#*.csv", detail.Source.Workbook)
	assert.Equal(t, "Activity", detail.Source.Worksheet)
	assert.Equal(t, "error[E0005]: custom check failed\nWorkbook: Test#*.csv\nWorksheet: Activity\nReason: awardId: 0 not found\n", serr.Error())
	data, marshalErr := json.Marshal(serr)
	require.NoError(t, marshalErr)
	assert.JSONEq(t, `{"details":[{"code":"E0005","description":"custom check failed","module":"default","message":"awardId: 0 not found","source":{"workbook":"Test#*.csv","worksheet":"Activity"}}]}`, string(data))
}

func TestCheckCompatibility(t *testing.T) {
	err := check.NewHub().CheckCompatibility("./testdata/", "./testdata1/", format.JSON,
		check.SkipLoadErrors(), check.BreakFailedCount(10),
		check.WithLoadOptions(load.IgnoreUnknownFields()))
	require.Error(t, err)
	serr := tableauapi.Inspect(err)
	var loads, compatibility int
	for _, detail := range serr.Details {
		if detail.Code == "E0005" {
			compatibility++
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

func TestLoadReferErrors(t *testing.T) {
	require.NoError(t, tableauapi.SetLang("en"))
	for _, tt := range []struct {
		name     string
		itemID   string
		itemCSV  string
		code     string
		referred bool
		wantText string
	}{
		{name: "valid value", itemID: "1", itemCSV: "ID\nuint32\nItem ID\n1\n"},
		{
			name: "missing value", itemID: "999", itemCSV: "ID\nuint32\nItem ID\n1\n", code: "E2002", referred: true,
			wantText: `error[E2002]: field value not in referred space
Workbook: Test#*.csv
Worksheet: ThemeConf
ReferWorkbook: Item#*.csv
ReferWorksheet: ItemConf
DataCellPos: C4
DataCell: 999
Reason: value "999" not in referred space "ItemConf.ID"
Help: correct value "999" or add it to one of the columns referenced by "ItemConf.ID"
`,
		},
		{
			name: "missing column", itemID: "1", itemCSV: "OtherID\nuint32\nItem ID\n1\n", code: "E2015", referred: true,
			wantText: `error[E2015]: referred sheet column not found
Workbook: Test#*.csv
Worksheet: ThemeConf
ReferWorkbook: Item#*.csv
ReferWorksheet: ItemConf
DataCellPos: C4
DataCell: 1
Reason: referred sheet column "ID" not found in workbook "Item#*.csv", worksheet "ItemConf"
Help: change "refer" prop or add referred sheet column "ID"
`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range map[string]string{
				"Test#ThemeConf.csv":   "Name,Value,ItemID\nstring,uint64,uint32\nName,Value,Item ID\nprimary,1," + tt.itemID + "\n",
				"Merge1#ThemeConf.csv": "Name,Value\nstring,uint64\nName,Value\nmerge1,1\n",
				"Merge2#ThemeConf.csv": "Name,Value\nstring,uint64\nName,Value\nmerge2,2\n",
				"Item#ItemConf.csv":    tt.itemCSV,
			} {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
			}
			err := check.NewHub(tableau.Filter(func(name string) bool { return name == "ThemeConf" })).Check(dir, format.CSV)
			if tt.code == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			serr := tableauapi.Inspect(err)
			require.Len(t, serr.Details, 1)
			detail := serr.Details[0]
			assert.Equal(t, tt.code, detail.Code)
			require.NotNil(t, detail.Source)
			assert.Equal(t, "Test#*.csv", detail.Source.Workbook)
			assert.Equal(t, "ThemeConf", detail.Source.Worksheet)
			assert.Equal(t, "Test#*.csv", detail.Source.PrimaryWorkbook)
			assert.Equal(t, "ThemeConf", detail.Source.PrimaryWorksheet)
			require.NotNil(t, detail.Source.Cell)
			assert.Equal(t, "C4", detail.Source.Cell.Position)
			assert.Equal(t, tt.itemID, detail.Source.Cell.Data)
			require.NotNil(t, detail.Field)
			assert.Equal(t, "protoconf.ThemeConf.Theme.item_id", detail.Field.Name)
			if tt.referred {
				assert.Equal(t, "Item#*.csv", detail.Source.ReferencedWorkbook)
				assert.Equal(t, "ItemConf", detail.Source.ReferencedWorksheet)
			}
			assert.Equal(t, tt.wantText, serr.Error())
		})
	}
}

func TestLoadMergerSheetErrors(t *testing.T) {
	require.NoError(t, tableauapi.SetLang("en"))
	for _, tt := range []struct {
		name      string
		badFile   string
		workbook  string
		worksheet string
		wantText  string
	}{
		{
			name: "primary sheet", badFile: "Test#ThemeConf.csv", workbook: "Test#*.csv", worksheet: "ThemeConf",
			wantText: `error[E2012]: invalid syntax of numerical value
Workbook: Test#*.csv
Worksheet: ThemeConf
DataCellPos: B4
DataCell: invalid
Reason: "invalid" cannot be parsed to numerical type "uint64", strconv.ParseUint: parsing "invalid": invalid syntax
Help: fill cell data with valid syntax of numerical type "uint64"
`,
		},
		{
			name: "merger sub-sheet", badFile: "Merge1#ThemeSub.csv", workbook: "Merge1#*.csv", worksheet: "ThemeSub",
			wantText: `error[E2012]: invalid syntax of numerical value
Workbook: Merge1#*.csv (Primary: Test#*.csv)
Worksheet: ThemeSub (Primary: ThemeConf)
DataCellPos: B4
DataCell: invalid
Reason: "invalid" cannot be parsed to numerical type "uint64", strconv.ParseUint: parsing "invalid": invalid syntax
Help: fill cell data with valid syntax of numerical type "uint64"
`,
		},
		{
			name: "primary workbook sub-sheet", badFile: "Test#ThemeSub.csv", workbook: "Test#*.csv", worksheet: "ThemeSub",
			wantText: `error[E2012]: invalid syntax of numerical value
Workbook: Test#*.csv
Worksheet: ThemeSub (Primary: ThemeConf)
DataCellPos: B4
DataCell: invalid
Reason: "invalid" cannot be parsed to numerical type "uint64", strconv.ParseUint: parsing "invalid": invalid syntax
Help: fill cell data with valid syntax of numerical type "uint64"
`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range map[string]string{
				"Test#ThemeConf.csv":  "Name,Value\nstring,uint64\nName,Value\nprimary,1\n",
				"Test#ThemeSub.csv":   "Name,Value\nstring,uint64\nName,Value\nsub,2\n",
				"Merge1#ThemeSub.csv": "Name,Value\nstring,uint64\nName,Value\nmerge1,3\n",
				"Merge2#ThemeSub.csv": "Name,Value\nstring,uint64\nName,Value\nmerge2,4\n",
			} {
				if name == tt.badFile {
					content = "Name,Value\nstring,uint64\nName,Value\nbad,invalid\n"
				}
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
			}
			err := check.NewHub(tableau.Filter(func(name string) bool { return name == "ThemeConf" })).Check(dir, format.CSV,
				check.WithLoadOptions(load.MaxErrorsPerSheet(5)))
			require.Error(t, err)
			serr := tableauapi.Inspect(err)
			require.Len(t, serr.Details, 1)
			detail := serr.Details[0]
			assert.Equal(t, "E2012", detail.Code)
			require.NotNil(t, detail.Source)
			assert.Equal(t, tt.workbook, detail.Source.Workbook)
			assert.Equal(t, tt.worksheet, detail.Source.Worksheet)
			assert.Equal(t, "Test#*.csv", detail.Source.PrimaryWorkbook)
			assert.Equal(t, "ThemeConf", detail.Source.PrimaryWorksheet)
			require.NotNil(t, detail.Source.Cell)
			assert.Equal(t, "B4", detail.Source.Cell.Position)
			assert.Equal(t, "invalid", detail.Source.Cell.Data)
			assert.Equal(t, tt.wantText, serr.Error())
			assert.Equal(t, tt.wantText, fmt.Sprint(serr))
			data, marshalErr := json.Marshal(serr)
			require.NoError(t, marshalErr)
			var decoded tableauapi.Error
			require.NoError(t, json.Unmarshal(data, &decoded))
			require.Len(t, decoded.Details, 1)
			assert.Equal(t, detail.Source, decoded.Details[0].Source)
			assert.Equal(t, tt.wantText, decoded.Error())
		})
	}
}

// TestLoadErrorText verifies one rendering flattens failures from separate loads.
func TestLoadErrorText(t *testing.T) {
	require.NoError(t, tableauapi.SetLang("en"))
	dir := t.TempDir()
	for name, content := range map[string]string{
		"Test#ChapterConf.csv": "ID,Name\nuint64,string\nChapter ID,Chapter Name\nbad-first,first\nbad-second,second\n",
		"Test#ThemeConf.csv":   "Name,Value\nstring,uint64\nName,Value\ntheme,bad-theme\n",
		"Merge1#ThemeConf.csv": "Name,Value\nstring,uint64\nName,Value\nmerge1,1\n",
		"Merge2#ThemeConf.csv": "Name,Value\nstring,uint64\nName,Value\nmerge2,2\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	err := check.NewHub(tableau.Filter(func(name string) bool {
		return name == "ChapterConf" || name == "ThemeConf"
	})).Check(dir, format.CSV, check.WithLoadOptions(load.MaxErrorsPerSheet(5)))
	require.Error(t, err)
	serr := tableauapi.Inspect(err)
	require.Len(t, serr.Details, 3)
	const chapterFirst = `error[E2012]: invalid syntax of numerical value
Workbook: Test#*.csv
Worksheet: ChapterConf
DataCellPos: A4
DataCell: bad-first
Reason: "bad-first" cannot be parsed to numerical type "uint64", strconv.ParseUint: parsing "bad-first": invalid syntax
Help: fill cell data with valid syntax of numerical type "uint64"
`
	const chapterSecond = `error[E2012]: invalid syntax of numerical value
Workbook: Test#*.csv
Worksheet: ChapterConf
DataCellPos: A5
DataCell: bad-second
Reason: "bad-second" cannot be parsed to numerical type "uint64", strconv.ParseUint: parsing "bad-second": invalid syntax
Help: fill cell data with valid syntax of numerical type "uint64"
`
	const theme = `error[E2012]: invalid syntax of numerical value
Workbook: Test#*.csv
Worksheet: ThemeConf
DataCellPos: B4
DataCell: bad-theme
Reason: "bad-theme" cannot be parsed to numerical type "uint64", strconv.ParseUint: parsing "bad-theme": invalid syntax
Help: fill cell data with valid syntax of numerical type "uint64"
`
	// Workers may finish in either order; numbering spans the complete result.
	wantText := "[1] " + chapterFirst + "\n[2] " + chapterSecond + "\n[3] " + theme
	if serr.Details[0].Source.Worksheet == "ThemeConf" {
		wantText = "[1] " + theme + "\n[2] " + chapterFirst + "\n[3] " + chapterSecond
	}
	assert.Equal(t, wantText, fmt.Sprint(serr))
	assert.Equal(t, "check failed, see errors below:\n"+wantText,
		fmt.Errorf("check failed, see errors below:\n%w", serr).Error())
}
