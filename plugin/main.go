package main

import (
	"github.com/flum1025/forceloader"
	"golang.org/x/tools/go/analysis"
)

type analyzerPlugin struct{}

func (a analyzerPlugin) GetAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		forceloader.Analyzer,
	}
}

var AnalyzerPlugin analyzerPlugin
