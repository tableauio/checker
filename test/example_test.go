package main

import (
	"fmt"

	"github.com/tableauio/checker/test/check"
	"github.com/tableauio/checker/test/customconf"
	"github.com/tableauio/checker/test/protoconf/tableau"
	tableauerrors "github.com/tableauio/tableau/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
)

// Example_check demonstrates how to run the checker against a directory of
// generated config outputs (JSON in this case) and surface a single,
// deterministic custom-check failure as a text-formatted error.
//
// BreakFailedCount(1) caps the run at the first failed messager so the example output
// stays stable regardless of how many other failures exist in the data set.
// CustomItemConf is registered via test/customconf and participates in load /
// ProcessAfterLoadAll (hence the "custom item conf processed" line).
func Example_check() {
	err := check.NewHub().Check(
		"./testdata/", format.JSON,
		check.BreakFailedCount(1),
		check.WithLoadOptions(load.IgnoreUnknownFields()),
	)
	if err != nil {
		fmt.Println(tableauerrors.Inspect(err))
	}
	// Output:
	// custom item conf processed
	// check ActivityConf failed: awardId: 0 not found
}

// Example_checkCompatibility demonstrates how to compare two snapshots of
// generated config outputs for compatibility regressions.
//
// The Filter selects ItemConf and its consumer ActivityConf so the example
// demonstrates a deterministic custom compatibility failure.
//
// The custom compatibility check on ActivityConf reports any ItemConf entry
// that existed in the old snapshot but disappears in the new one.
func Example_checkCompatibility() {
	allowed := map[string]bool{"ItemConf": true, "ActivityConf": true}
	err := check.NewHub(tableau.Filter(func(name string) bool {
		return allowed[name]
	})).CheckCompatibility(
		"./testdata/", "./testdata1/", format.JSON,
		check.BreakFailedCount(10),
		check.WithLoadOptions(load.IgnoreUnknownFields()),
	)
	if err == nil {
		fmt.Println("compatible")
		return
	}
	fmt.Println(tableauerrors.Inspect(err))
	// Output:
	// check compatibility of ActivityConf failed: ItemConf incompatible: 5 item id(s) removed in new version: [2 3 2001 2002 2003]
}

// Example_customConf shows reading a derived messager after Check. The hub
// loads all registered messagers (including CustomItemConf); Check may still
// fail on ActivityConf, but ProcessAfterLoadAll has already built the index.
func Example_customConf() {
	hub := check.NewHub()
	_ = hub.Check(
		"./testdata/", format.JSON,
		check.BreakFailedCount(1),
		check.WithLoadOptions(load.IgnoreUnknownFields()),
	)
	conf := tableau.GetMessager[*customconf.CustomItemConf](hub.GetMessagerMap())
	fmt.Println(conf.GetSpecialItemName())
	// Output:
	// custom item conf processed
	// coin1
}
