package forceloader_test

import (
	"testing"

	"github.com/flum1025/forceloader"
	"github.com/gostaticanalysis/testutil"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	forceloader.SetRestrictedFieldSuffix("UseCase")
	forceloader.SetIgnoreResolvers("queryResolver,mutationResolver")

	testdata := testutil.WithModules(t, analysistest.TestData(), nil)
	analysistest.Run(t, testdata, forceloader.Analyzer, "a")
}
