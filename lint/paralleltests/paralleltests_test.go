package paralleltests

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()
	serial, err := filepath.Abs("testdata/serial-tests.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := Analyzer.Flags.Set("serial", serial); err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
}
