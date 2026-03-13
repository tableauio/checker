package main

import (
	"testing"

	"github.com/tableauio/checker/test/check"
	"github.com/tableauio/checker/test/protoconf/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
)

func TestCheck(t *testing.T) {
	err := check.NewHub(tableau.Filter(Filter)).Check("./testdata/", format.JSON,
		check.BreakFailedCount(2),
		check.WithLoadOptions(load.IgnoreUnknownFields()),
	)
	if err != nil {
		t.Errorf("check failed, see errors below:\n%v", err)
	}
}

func TestCheckCompatibility(t *testing.T) {
	err := check.NewHub(tableau.Filter(Filter)).CheckCompatibility("./testdata/", "./testdata1/", format.JSON,
		check.SkipLoadErrors(),
		check.BreakFailedCount(2),
		check.WithLoadOptions(load.IgnoreUnknownFields()),
	)
	// testdata1 contains intentionally broken/incompatible data, so errors are expected.
	if err == nil {
		t.Errorf("check compatibility should have failed with testdata1, but got no error")
	} else {
		t.Logf("check compatibility returned expected errors:\n%v", err)
	}
}
