package main

import (
	"fmt"

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
// To keep the example output deterministic, the Filter narrows the run to
// the messagers whose error messages are stable single-liners (ItemConf as
// the dependency + ActivityConf as the consumer that runs the custom
// CheckCompatibility). This avoids pulling in messagers like ThemeConf
// whose load errors include a multi-line file excerpt that would clutter
// the example.
//
// The custom compatibility check on ActivityConf reports any ItemConf entry
// that existed in the old snapshot but disappears in the new one.
func Example_checkCompatibility() {
	allowed := map[string]bool{"ItemConf": true, "ActivityConf": true}
	filter := func(name string) bool { return allowed[name] && Filter(name) }

	err := check.NewHub(tableau.Filter(filter)).CheckCompatibility(
		"./testdata/", "./testdata1/", format.JSON,
		check.BreakFailedCount(10),
		check.WithLoadOptions(load.IgnoreUnknownFields()),
	)
	if err == nil {
		fmt.Println("compatible")
		return
	}
	fmt.Println(err)
	// Output:
	// error: workbook Test#*.csv, worksheet Activity, custom check failed: ItemConf incompatible: 5 item id(s) removed in new version: [2 3 2001 2002 2003]
}
