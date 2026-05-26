package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/tableauio/checker/test/check"
	"github.com/tableauio/checker/test/protoconf/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
)

// Example_check demonstrates how to run the checker against a directory of
// generated config outputs (JSON in this case) and surface a single,
// deterministic custom-check failure as a text-formatted error.
//
// BreakFailedCount(1) caps the run at the first issue so the example output
// stays stable regardless of how many other issues exist in the data set.
func Example_check() {
	err := check.NewHub(tableau.Filter(Filter)).Check(
		"./testdata/", format.JSON,
		check.BreakFailedCount(1),
		check.WithLoadOptions(load.IgnoreUnknownFields()),
	)
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// error: workbook Test#*.csv, worksheet Activity, custom check failed: awardId: 0 not found
}

// Example_checkCompatibility demonstrates how to compare two snapshots of
// generated config outputs for compatibility regressions.
//
// SkipLoadErrors lets the run proceed even if individual messagers fail to
// load, and BreakFailedCount caps the number of issues collected per run.
//
// Issues from CheckCompatibility are gathered concurrently across messagers,
// so their order is not deterministic. The example unwraps the returned
// *check.Error, sorts the issues by (workbook, worksheet, kind), and prints
// a one-line summary per issue (kind + location + first line of message) so
// the "Output:" assertion stays stable across runs.
func Example_checkCompatibility() {
	err := check.NewHub(tableau.Filter(Filter)).CheckCompatibility(
		"./testdata/", "./testdata1/", format.JSON,
		check.SkipLoadErrors(),
		check.BreakFailedCount(10),
		check.WithLoadOptions(load.IgnoreUnknownFields()),
	)
	if err == nil {
		fmt.Println("compatible")
		return
	}

	var ce *check.Error
	if !errors.As(err, &ce) {
		fmt.Println(err)
		return
	}

	issues := append([]*check.Issue(nil), ce.Issues...)
	sort.SliceStable(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if x, y := a.Workbook.GetName(), b.Workbook.GetName(); x != y {
			return x < y
		}
		if x, y := a.Worksheet.GetName(), b.Worksheet.GetName(); x != y {
			return x < y
		}
		return a.Kind < b.Kind
	})

	for _, issue := range issues {
		// Use only the first line of the message: some custom checks
		// (e.g. ItemConf) include a prototext dump whose map-field
		// ordering is not stable across runs.
		firstLine := strings.SplitN(issue.Message, "\n", 2)[0]
		fmt.Printf("[%s] %s/%s: %s\n",
			issue.Kind,
			issue.Workbook.GetName(),
			issue.Worksheet.GetName(),
			firstLine)
	}
	// Output:
	// [compatibility] Test#*.csv/Activity: custom check failed: load ItemConf successfully even it's checker is not registered
	// [load] Test#*.csv/ChapterConf: load failed: failed to read file: testdata1/ChapterConf.json: open testdata1/ChapterConf.json: no such file or directory
	// [load] Test#*.csv/ThemeConf: load failed: failed to unmarshal file "testdata1/ThemeConf.json" to message "protoconf.ThemeConf": proto: (line 9:22): invalid value for uint64 field value: "invalid-type-not-integer"
}
