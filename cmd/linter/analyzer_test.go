package main

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestPanicExitAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer, "./...")
}
